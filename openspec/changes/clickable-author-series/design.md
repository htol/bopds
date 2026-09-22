# Design

## Context

See `proposal.md` — Why. Current state that constrains the design:

- Grouped book lists (`/api/books?startsWith=`, `/api/authors/{id}/books` → `service.BookGroup`)
  already serialize author IDs (JSON key `ID`, from `book.Author`) and `series.series_id`, but
  the frontend renders them as plain text.
- Flat search results (`repo.SearchBooks` → `book.BookSearchResult`) have a single concatenated
  `author` string and `series_name`/`series_no` — no IDs at all.
- The web UI has no router: `LibraryTabs.vue` switches views by tab, keeps hash per tab, and
  already does one cross-view hop (`GenresView` emits `select-genre` → Search tab opens with a
  pending query). `AuthorsView` has an in-memory detail mode (no deep link).
- The bundled SQLite (mattn/go-sqlite3 v1.14.14) rejects `ORDER BY` inside `DISTINCT` aggregates
  and multi-argument `DISTINCT` aggregates (verified with a throwaway test, since removed).

## Goals / Non-Goals

**Goals:**

- Every book card can navigate to its author's full book list and to the series' book list.
- Guaranteed alignment between displayed author names and their IDs in search results.
- Series book list behaves like the other grouped lists (duplicates merged, downloads work).

**Non-Goals:**

- OPDS series navigation (OPDS feeds stay unchanged).
- A series index/letter-browse tab; the series view is reachable only from a card.
- Deep-linkable URLs or hash entries for author/series detail (consistent with the existing
  author detail, which is in-memory only).
- Any change to search semantics, ranking, or existing response fields.

## Decisions

### D1: Search payload gains `author_ids` and `series_id`

`book.BookSearchResult` gets `AuthorIDs []int64` (`author_ids`) and `SeriesID int64`
(`series_id,omitempty`, 0 = no series).

**Alignment problem and solution.** The natural approach — a second aggregate
`group_concat(DISTINCT a.author_id)` next to the existing name aggregate — is unsafe: two
separate aggregates have no guaranteed shared order, and the bundled SQLite cannot pin the
order (no `ORDER BY` with `DISTINCT`, no `DISTINCT` with a custom separator — both verified).
Instead, replace the author aggregate with a pre-aggregated derived table over
`book_authors` + `authors` only (one row per book-author pair, so no `DISTINCT` is needed and
a custom separator is allowed):

```sql
LEFT JOIN (
    SELECT ba.book_id,
           group_concat(a.author_id || char(31) ||
             (a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '')), char(30))
             AS author_combined
    FROM book_authors ba
    JOIN authors a ON a.author_id = ba.author_id
    GROUP BY ba.book_id
) aa ON aa.book_id = b.book_id
```

One aggregate → one order → IDs structurally aligned with names. Go splits on `char(30)`,
then each entry at the first `char(31)`, and rebuilds the visible `author` string by joining
names with `,` — byte-identical to today's output. Control-character separators cannot occur
in FB2 names. A derived table also removes the author rows from the genre-join multiplication
the old `DISTINCT` guarded against.

`series_id` is a scalar column from the existing `book_series` join (nullable, scanned like
`series_no`). Edge case preserved as-is: a book linked to several series already produces
several result rows (existing `GROUP BY` includes series columns).

### D2: New endpoint `GET /api/series/{id}/books`

- `repo`: `GetBooksBySeriesID(id)` mirrors `GetBooksByAuthorID` (same `booksMap` multi-author
  dedup pattern), `ORDER BY bs.series_no, b.title`; resolves the series first and returns
  `repo.ErrNotFound` when no series row has the ID.
- `service`: `GetBooksBySeriesIDGrouped(ctx, id)` wraps it with the existing `groupBooks`
  (interface `Repository` gains the method — update test mocks).
- `api`: register `mux.Handle("GET /api/series/{id}/books", withCORS(...))` using
  `r.PathValue("id")` (Go 1.26 ServeMux patterns, same as OPDS routes). Malformed ID → 400
  (`respondWithValidationError`), unknown series → 404 (`respondWithError`, matching
  `getAuthorByIDHandler`).

### D3: Card interaction — per-author links, series link, event bubbling

`UniversalBookCard.vue`:

- The author line renders one link per author (separator `, `), replacing the single
  highlighted string. Each segment is highlighted individually (existing `highlightMatches`),
  so search highlighting still works inside a link. Flat cards zip `author_ids` with the
  `author` string split by `, `; grouped cards map over the `authors` array (JSON key `ID`).
  Click emits `author-click` with `{ id, name }` and uses `.stop` so neither the card's click
  nor any download fires.
- The series span becomes a link emitting `series-click` with `{ id, name }` (`series_id` /
  `series.series_id`; name without the `#no` suffix), also `.stop`.

Parents re-emit the events (`SearchView`, `BooksView`, `AuthorsView`) up to `LibraryTabs`,
following the existing `select-genre` pattern.

### D4: Navigation targets

- **Author click** → `LibraryTabs` switches to the Authors tab with a pending author ID
  (new prop on `AuthorsView`): the view fetches the author via the existing
  `api.getAuthorById` and enters its existing detail mode. Clicking an author while already
  in that author's detail simply reloads it — harmless, no special-casing.
- **Series click** → `LibraryTabs` renders a new `SeriesView.vue` in place of the tab content
  (back button returns to the previously active tab). The view receives `{ id, name }` from
  the click (name is always available in both card shapes), loads
  `api.getBooksBySeries(id)` — new client method — and renders `UniversalBookCard` list with
  downloads, mirroring `AuthorsView` detail (loader, empty state, back button).
- No hash/deep-link changes; back navigation is the in-page button, same as author detail.

## Risks / Trade-offs

- [Combined-string parsing in Go relies on `char(30)`/`char(31)` not appearing in author
  names] → Non-printable separators cannot be produced by the FB2 parser; a stray one would
  at worst render one odd name, not break the API.
- [`author` string rebuilt in Go could differ from today's SQL output] → Same concatenation
  expression and same `,` join; covered by an assertion in the repo search test comparing the
  old and new outputs for the fixture books.
- [Duplicate full names of different authors now appear twice in the `author` string
  (today's `DISTINCT` collapsed them)] → More correct: each clickable name maps to its own
  ID; structurally required for alignment.
- [Repository interface change breaks test mocks] → Add the method to each inline mock in
  `service` tests; compile errors pinpoint them.
- [Series books ordering by `series_no` puts books without a number first] → Matches reader
  expectation for series; ties fall back to title.

## Migration Plan

Additive only: new response fields, one new endpoint, frontend-only navigation. Deploy and
roll back by redeploying the previous binary; no schema or data changes. Old frontend bundles
ignore the new fields.

## Open Questions

None blocking. (Whether to later add OPDS series feeds or a series index can be decided in
future changes; neither affects this design.)
