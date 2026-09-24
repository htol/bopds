# Spec Delta

## Purpose

Defines the behavioral contract of the book search endpoint for interactive use: response
time that stays usable on large libraries regardless of how many books the library holds,
and treatment of client-initiated cancellation as a normal event rather than a server
error.

## ADDED Requirements

### Requirement: Search response time scales with matches, not library size

The search endpoint (`GET /api/search`) SHALL answer within an interactive timeframe (2
seconds) on a library of at least one million indexed books for a query that matches a
small number of books. The work performed for a search SHALL be bounded by the number of
matched books and the requested page size, not by the total size of the library.

#### Scenario: Prefix search on a million-book library

- **WHEN** the client searches a 1,000,000-book library with a selective query matching
  about 200 books
- **THEN** the response arrives within 2 seconds

#### Scenario: Result content is unchanged by the performance contract

- **WHEN** the client runs the same query against the same library before and after this
  requirement is in force
- **THEN** the returned fields (including `author_ids` aligned with the author names) and
  their order are identical

### Requirement: Client cancellation is not a server error

When a client disconnects or aborts a search request before it completes, the server SHALL
NOT log the cancellation at ERROR level and SHALL NOT attempt a 500 error response for it.
Cancellation SHALL be recorded as an informational event at most.

#### Scenario: Client aborts while search is running

- **WHEN** the client closes the connection while a search request is still being processed
- **THEN** no ERROR-level entry is logged for that request and no 500 response is emitted

#### Scenario: Genuine failures still surface

- **WHEN** a search fails for a reason other than client cancellation (for example a
  database error)
- **THEN** the failure is logged at ERROR level and a 500 response is attempted
