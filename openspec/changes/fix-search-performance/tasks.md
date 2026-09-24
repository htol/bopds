# Tasks

## 1. Query fix (repo)

- [x] 1.1 In `repo/search.go` replace the `aa` derived table with the correlated scalar
  subquery from design D1 (keep the inner DISTINCT, `char(30)`/`char(31)` encoding, and the
  `substr(...)` ORDER BY); verify `go test ./repo/` passes with the existing search tests
  (output and `author_ids` alignment unchanged)
- [x] 1.2 Add a repo test for a book with duplicate `book_authors` pairs asserting one
  author entry and aligned `author_ids` (the case that motivated the derived table);
  verify the new test fails against the old query shape is unnecessary — it must pass on
  the new shape
- [x] 1.3 Measure the fixed query against the production DB copy (`sqlite3` timing for
  `риордан*`): confirm the interactive bound from the spec (sub-second at 130 matches)
  — measured 2026-09-24 on the copy from the bopds container (1.17M books,
  5.75M `book_authors`, 216 matched books): fixed query 11–16 ms with LIMIT 20
  vs 5.9 s for the old derived-table shape; outputs byte-identical

## 2. Cancellation hygiene (api)

- [x] 2.1 In the search handler (`api/rest.go`) special-case
  `errors.Is(err, context.Canceled)`: Info log, no response write; verify with an
  `httptest` request whose context is canceled mid-search that no 500 is written and no
  ERROR is logged
- [x] 2.2 Downgrade the post-encode "Failed to encode search results" log from ERROR to
  Debug in the search handler; verify by existing api tests still passing

## 3. Regression guard

- [x] 3.1 Run `make lint` and `make test`; fix any findings
