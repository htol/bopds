# Proposal

## Why

Book cards show the author and the series (when present) as plain text. The reader who finds
one book cannot jump to the rest of that author's books or the rest of the series — the data
to do it already exists in the database, but neither the API nor the UI exposes it as a
navigable link. Every card should be an entry point for browsing the library.

## What Changes

- Flat search results (`GET /api/search`) additionally return `author_ids` (aligned with the
  existing concatenated `author` string) and `series_id` (when the book belongs to a series).
- New endpoint `GET /api/series/{id}/books` returns all non-deleted books of a series,
  grouped by author + title across libraries (same group shape as `/api/books`).
- In every book card (search results, books by letter, author detail), each author name and
  the series label become clickable links.
- An author link opens the author's complete book list (the existing Authors-tab detail view).
- A series link opens a new series view listing all books of that series, ordered by series
  number.
- All payload changes are additive; no existing endpoint or field changes (**no breaking
  changes**).

## Capabilities

### New Capabilities
- `book-card-navigation`: navigating from a book card to the complete book list of its author
  or series — the identifiers in API payloads, the books-by-series endpoint, and the
  clickable links in the web UI.

### Modified Capabilities

(none — the project has no specs yet)

## Impact

- **Backend**: `book` (new `BookSearchResult` fields), `repo` (search query joins, new
  books-by-series query), `service` (books-by-series with grouping), `api` (new route and
  handler).
- **Frontend**: `UniversalBookCard.vue` (clickable author/series, new events),
  `LibraryTabs.vue` (cross-view navigation), a new series detail view, event wiring in
  `SearchView.vue` / `BooksView.vue` / `AuthorsView.vue`, `api.js` (new client method).
- **Tests**: handler, repo and service tests follow the existing table-driven style; the
  frontend has no test setup (manual verification with `make scan-fixture` data).
