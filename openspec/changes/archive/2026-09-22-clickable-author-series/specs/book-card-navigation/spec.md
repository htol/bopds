# Spec Delta

## Purpose

Lets a reader open the complete book list of an author or of a series directly from a book
card, by making the author and series fields in every card navigable in the web UI, and by
serving the data those links need over the REST API.

## ADDED Requirements

### Requirement: Search results identify authors and series

The search endpoint (`GET /api/search`) SHALL return, for every result, the numeric IDs of
the book's authors (`author_ids`) in the same order the authors are listed in that result,
and the numeric ID of the book's series (`series_id`) when the book belongs to one. These
fields are additive; existing response fields SHALL NOT change.

#### Scenario: Result with a series and multiple authors

- **WHEN** a search matches a book that has two authors and belongs to a series
- **THEN** the result contains `author_ids` with two IDs matching the order of the names in
  the result's author field, and `series_id` matching that book's series

#### Scenario: Result without a series

- **WHEN** a search matches a book that has one author and no series
- **THEN** the result contains `author_ids` with one ID and carries no `series_id` value

### Requirement: Books by series endpoint

The system SHALL serve the books of a series at `GET /api/series/{id}/books`. The response
SHALL contain all non-deleted books of the series, with duplicates across libraries merged
into one entry per author + title (the same grouping as the other grouped book lists),
ordered by series number and then by title. A malformed ID SHALL be rejected with 400; an
ID that matches no series SHALL return 404.

#### Scenario: Series with books in several libraries

- **WHEN** the client requests the books of a series that contains the same book in two
  libraries
- **THEN** the response contains one entry for that book with one downloadable copy per
  library, ordered by series number

#### Scenario: Unknown series

- **WHEN** the client requests `/api/series/999999/books` and no series has that ID
- **THEN** the response status is 404

### Requirement: Author names in book cards are links

In every book card in the web UI (search results, books-by-letter list, author detail), each
displayed author name SHALL act as a link. Activating it SHALL open the complete book list
of that author — the same view reached from the Authors tab. The activation SHALL NOT
trigger the card's own click behavior or any download action.

#### Scenario: Click an author in a search result

- **WHEN** the reader clicks an author name in a book card shown in search results
- **THEN** the author's book list opens, showing that author's books grouped by author +
  title

#### Scenario: Link activation stays on the link

- **WHEN** the reader clicks an author name or series label in a card
- **THEN** no card-level click handler and no download is triggered

### Requirement: Series labels in book cards are links

Whenever a book card shows a series, the series label SHALL act as a link. Activating it
SHALL open a series view listing all books of that series ordered by series number, with a
way to go back to the previous view. Books without a series SHALL render exactly as before.

#### Scenario: Click a series label

- **WHEN** the reader clicks the series label in a card
- **THEN** a series view opens showing every book of that series ordered by series number,
  each book rendered as a card with download actions

#### Scenario: Card without a series

- **WHEN** a card is rendered for a book without a series
- **THEN** no series link is shown and the card layout is unchanged
