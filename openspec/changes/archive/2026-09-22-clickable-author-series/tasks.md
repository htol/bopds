# Tasks

## 1. Backend: search payload identifiers

- [x] 1.1 Add `AuthorIDs []int64` (`author_ids`) and `SeriesID int64` (`series_id,omitempty`)
  to `book.BookSearchResult`; verify the package builds with `go build ./...`
- [x] 1.2 Rework the author aggregate in `repo.SearchBooks` into the pre-aggregated
  `book_authors`+`authors` derived table with the combined `char(31)`/`char(30)` aggregate
  (design D1), scan and split it in Go to fill `Author` and `AuthorIDs`, and select
  `bs.series_id` for `SeriesID`; verify `go test -tags "sqlite_omit_load_extension,fts5"
  ./repo/` passes
- [x] 1.3 Extend `repo/search_test.go`: multi-author fixture asserts `author_ids` aligns with
  the names in the author string, series fixture asserts `series_id`, and a no-series book
  asserts `series_id` stays empty; verify with `go test -tags
  "sqlite_omit_load_extension,fts5" -run TestSearch ./repo/ -v`

## 2. Backend: books-by-series endpoint

- [x] 2.1 Implement `repo.GetBooksBySeriesID(id)` (design D2: mirror `GetBooksByAuthorID`,
  series lookup → `repo.ErrNotFound` when absent, `ORDER BY bs.series_no, b.title`); verify
  with a new `repo` test covering found, empty-series, and unknown-ID cases
- [x] 2.2 Add `GetBooksBySeriesID(ctx, id)` and `GetBooksBySeriesIDGrouped(ctx, id)` to
  `service`, extend the `Repository` interface and every inline mock in `service` tests;
  verify `go test ./service/`
- [x] 2.3 Register `GET /api/series/{id}/books` in `api/handler.go` (withCORS, `PathValue`)
  with a handler returning grouped books, 400 on malformed ID, 404 on unknown series
  (design D2); verify with a new case in `api/api_test.go` covering 200 (grouped, ordered by
  series number), 400, and 404
- [x] 2.4 Run `make lint` and `make test`; fix any findings

## 3. Frontend: clickable card fields

- [x] 3.1 Add `getBooksBySeries(seriesId)` to `frontend/src/api.js`; verify the call shape
  matches the endpoint from task 2.3 by loading it in the browser devtools
- [x] 3.2 In `UniversalBookCard.vue`: render one link per author (flat cards zip
  `author_ids` with the `author` string split by `, `; grouped cards map the `authors`
  array), keep per-segment search highlighting, make the series label a link, emit
  `author-click` / `series-click` with `{ id, name }` and `.stop` (design D3); verify
  manually that clicks do not trigger card click or downloads
- [x] 3.3 Wire re-emit of `author-click` / `series-click` through `SearchView.vue`,
  `BooksView.vue`, `AuthorsView.vue` up to `LibraryTabs.vue`; verify with Vue devtools that
  the events reach `LibraryTabs` from each view

## 4. Frontend: navigation targets

- [x] 4.1 `LibraryTabs.vue`: on `author-click` switch to the Authors tab with a pending
  author id prop; extend `AuthorsView.vue` to enter detail mode from that id via
  `api.getAuthorById` (design D4); verify manually: author link in a search result opens the
  author's book list
- [x] 4.2 Create `SeriesView.vue` (back button, series name header, loader, empty state,
  `UniversalBookCard` list fed by `api.getBooksBySeries`) and render it from `LibraryTabs`
  in place of tab content on `series-click`, returning to the previous tab on back (design
  D4); verify manually: series link opens all books of the series ordered by series number,
  back returns to the originating view

## 5. Integration verification

- [x] 5.1 `make scan-fixture` then `make serve` (or `npm run dev`): check clickable
  author/series in all three card contexts (search results, books by letter, author detail),
  cards without series render unchanged, downloads still work from the series view
- [x] 5.2 `make build` and `make test` pass cleanly on the final tree
