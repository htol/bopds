# Proposal

## Why

Commit 9eb41d2 (clickable author/series navigation) regressed `GET /api/search` from ~80 ms
to ~9 s on a production-size library (1.17M books): the author aggregate became a derived
table that materializes ALL `book_authors` pairs (5.7M rows -> 1.17M groups) on every
request. Interactive search is unusable — requests queue for 20-68 s, clients give up, and
every cancellation is logged as ERROR with a 500 response. Both defects are user-visible
today (error banner in the UI, ERROR noise in logs).

## What Changes

- `repo.SearchBooks` author aggregation moves from the whole-table derived table to a
  correlated scalar subquery evaluated per matched book. Measured on the production DB:
  8.9 s -> 15 ms for `риордан*` (130 matched books). The pre-existing `I_book_id` index
  serves the per-book lookups; no new indexes, no migration.
- Client-canceled requests (`context.Canceled`) stop being treated as server errors: no
  ERROR log line, no 500 response attempt for the search endpoint. Response-encode
  failures on dead connections are downgraded from ERROR.
- No changes to the API shape, response fields, ranking, or schema. The alignment contract
  of `book-card-navigation` (author_ids order == author names order) is preserved.

## Capabilities

### New Capabilities

- `book-search`: behavioral contract for the search endpoint — response time that stays
  interactive on large libraries, and client-cancellation handling that is not a server
  error.

### Modified Capabilities

(none — the `book-card-navigation` requirement "Search results identify authors and
series" is preserved unchanged by this fix)

## Impact

- `repo/search.go` (query shape), `api/rest.go` (cancellation handling, encode log level),
  `repo/search_test.go`, `api/api_test.go`.
- OPDS search (`/opds/search`) goes through the same `svc.SearchBooks` and benefits
  automatically.
- Deployment: new image only; no schema or data changes.
