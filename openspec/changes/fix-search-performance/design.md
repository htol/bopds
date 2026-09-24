# Design

## Context

`repo.SearchBooks` builds one SQL query: FTS match -> join `books` -> author aggregate ->
series/genre joins -> GROUP BY -> ORDER BY -> LIMIT/OFFSET. The FTS stage is fast (1 ms,
selective). The regression is isolated to the author aggregate: since 9eb41d2 it is an
uncorrelated derived table that SQLite materializes fully before joining, so its cost is
O(total library), not O(matched books).

Constraints that any fix must keep (from the archived design of 9eb41d2 and its spec):

- `author_ids` and the visible author names must come from ONE aggregate, so IDs and names
  are structurally aligned (bundled SQLite cannot pin order across two aggregates).
- The combined format `id<char(31)>name` joined by `char(30)`, and the Go decoder
  `splitAuthorCombined`, were chosen because control characters cannot occur in FB2 names.
- Duplicate (book_id, author_id) pairs in `book_authors` (no unique constraint) must
  collapse to one entry.
- Response fields, ranking, and the `author` string output stay byte-identical.

Indexes on the live DB: `I_book_id`, `I_author_id` on `book_authors` exist (verified on the
production volume).

Measurements on the production DB (1.17M books, 5.7M `book_authors` rows, query
`риордан*`, 130 matched books):

| Query shape                                  | Time    |
|----------------------------------------------|---------|
| Current derived table (materializes all)     | 8 900 ms|
| Pre-9eb41d2 direct join                      | 80 ms   |
| Candidate A: `group_concat(DISTINCT combined)` | 21 ms |
| **Candidate B: correlated scalar subquery**  | 15 ms   |

## Goals / Non-Goals

**Goals:**

- Search response time proportional to matched books, not library size.
- Client cancellation is not an error: no ERROR log, no 500 attempt.
- Keep every output byte and the alignment contract intact.

**Non-Goals:**

- Duplicate result rows for books in several series (pre-existing; needs a product
  decision on which series to show — tracked separately).
- Frontend changes (debounce/abort logic stays as is; with a fast endpoint it stops
  mattering).
- New indexes or schema changes.

## Decisions

### D1: Correlated scalar subquery per matched book (candidate B)

Replace the derived table with a scalar subquery in the SELECT list, mirroring the pattern
`RebuildFTSIndex` already uses:

```sql
(SELECT group_concat(author_id || char(31) || author_name, char(30))
 FROM (SELECT DISTINCT ba.author_id,
              a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '') AS author_name
       FROM book_authors ba
       JOIN authors a ON ba.author_id = a.author_id
       WHERE ba.book_id = b.book_id)) AS author_combined
```

- Per matched book this is one `I_book_id` range scan; 130 lookups cost milliseconds.
- The inner `DISTINCT` keeps collapsing duplicate pairs; one aggregate keeps the id/name
  alignment; `char(30)`/`char(31)` and `splitAuthorCombined` stay untouched.
- `ORDER BY` keeps the current `substr(author_combined, instr(...))` form, evaluated over
  the matched rows only. `LIMIT` does not reduce the number of subquery evaluations (all
  matched rows are ordered first) — irrelevant at this scale.

Alternative rejected — candidate A (`group_concat(DISTINCT author_id || char(31) || name)`
in the outer query, 21 ms): forces the default `,` separator (custom separators are
incompatible with DISTINCT), weakening the "separator cannot occur in names" invariant to
"names contain no commas", and requires reworking the Go splitter. Same speed class, worse
invariants.

Alternative rejected — `NOT MATERIALIZED` CTE hint: relies on planner behavior of a
specific bundled SQLite version; the correlated form states the intent directly.

### D2: Client cancellation is not a server error

In the search handler (`api/rest.go`), before `respondWithError`:

- `errors.Is(err, context.Canceled)` -> log at Info ("client canceled search") and return
  without writing a status (the client is gone; if it is not, the connection is about to
  be).
- The existing "Failed to encode search results" ERROR after `json.NewEncoder` failures
  (dead sockets) is downgraded to Debug: with D1 searches finish before clients give up,
  and encode failures on dead connections are not actionable.

Scope: the search handler only. Other slow endpoints (e.g. `/api/books?startsWith=`) can
adopt the same pattern later; their cancellation noise is not part of this regression.

### D3: Test guard against re-introducing O(library) work

The repo search tests already compare query output against expectations on the shared
fixture. Timing on the small CI fixture cannot catch a whole-table aggregate, so the guard
is structural: keep the fixture small and add a test asserting results (and
`author_ids` alignment) for a book with duplicate `book_authors` pairs — the case that
motivated the derived table — proving the correlated `DISTINCT` covers it.

## Risks / Trade-offs

- [Correlated subquery is evaluated per row before ORDER BY/LIMIT] -> Cost is bounded by
  matched books; acceptable at any realistic match count (even 10k matches stay < 1 s).
- [Planner picks a worse plan for some query shape (e.g. `fields=title:*`)] -> The author
  subquery shape is identical regardless of the FTS restriction; covered by the existing
  fields-restricted tests.
- [Cancellation branch hides real errors] -> Only `context.Canceled` is special-cased;
  deadlines and other errors keep the ERROR + 500 path.

## Migration Plan

None. Single binary/image change; rollback = previous image. No schema or data changes.
