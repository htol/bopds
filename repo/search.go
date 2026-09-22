package repo

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
)

func escapeFTS5Query(query string) string {
	query = strings.TrimSpace(query)

	space := regexp.MustCompile(`\s+`)
	query = space.ReplaceAllString(query, " ")
	query = strings.TrimSpace(query)

	escaped := strings.ReplaceAll(query, "\"", "\"\"")
	escaped = strings.ReplaceAll(escaped, "-", " ")
	escaped = strings.ReplaceAll(escaped, "'", "")
	escaped = strings.ReplaceAll(escaped, "(", "")
	escaped = strings.ReplaceAll(escaped, ")", "")
	escaped = strings.ReplaceAll(escaped, "{", "")
	escaped = strings.ReplaceAll(escaped, "}", "")

	return strings.TrimSpace(escaped)
}

// SearchBooks performs full-text search across book titles and authors
// Uses FTS5 for fast, ranked search results
// Optimized with single query including author JOIN (fixes N+1 query issue)
func (r *Repo) SearchBooks(ctx context.Context, query string, limit, offset int, fields []string, languages []string) ([]book.BookSearchResult, error) {
	// Validate query
	if query == "" {
		return []book.BookSearchResult{}, nil
	}

	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return []book.BookSearchResult{}, nil
	}

	// Escape FTS5 special characters to prevent injection
	escapedQuery := escapeFTS5Query(cleanQuery)
	var ftsQuery string

	// If specific fields are requested, restrict the search
	if len(fields) > 0 {
		var parts []string
		for _, f := range fields {
			parts = append(parts, fmt.Sprintf("%s:%s*", f, escapedQuery))
		}
		// Combine with OR: (title:foo* OR author:foo*)
		// We use standard FTS5 syntax
		ftsQuery = fmt.Sprintf("(%s)", strings.Join(parts, " OR "))
	} else {
		// Default search across all columns
		ftsQuery = escapedQuery + "*"
	}

	// Base arguments required for building the query b.c. sql doesn't support slice arguments as IN clause
	var args []interface{}
	args = append(args, ftsQuery)

	// Search FTS5 table and join back to books table for full details
	// Uses book_id column for direct, accurate mapping
	//
	// Authors come from a pre-aggregated derived table: one row per book with
	// "id<char(31)>name" entries joined by char(30). A single aggregate
	// guarantees the IDs and the names share one order (two separate
	// group_concat calls have no guaranteed shared order, and the bundled
	// SQLite cannot pin it: no ORDER BY inside DISTINCT aggregates, no
	// multi-argument DISTINCT). The inner DISTINCT collapses duplicate
	// book-author pairs (book_authors has no unique constraint). The derived
	// table also removes the author rows from the genre-join multiplication
	// the old DISTINCT guarded against.
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT
			b.book_id,
			b.title,
			b.lang,
			b.archive,
			b.filename,
			b.file_size,
			b.deleted,
			s.name as series_name,
			bs.series_no,
			bs.series_id,
			fts.rank,
			aa.author_combined as author,
			group_concat(distinct g.display_name) as genres,
			l.name as library,
			COALESCE(l.display_name, l.name) as library_display_name
		FROM books_fts fts
		JOIN books b ON fts.book_id = b.book_id
		LEFT JOIN (
			SELECT book_id,
				   group_concat(author_id || char(31) || author_name, char(30))
					 AS author_combined
			FROM (
				-- One row per book-author pair: book_authors has no unique
				-- constraint, so duplicate pairs (same author listed twice)
				-- must collapse here to keep one entry per author
				SELECT DISTINCT ba.book_id, ba.author_id,
						a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '') AS author_name
				FROM book_authors ba
				JOIN authors a ON ba.author_id = a.author_id
			)
			GROUP BY book_id
		) aa ON aa.book_id = b.book_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		LEFT JOIN book_genres bg ON b.book_id = bg.book_id
		LEFT JOIN genres g ON bg.genre_id = g.genre_id
		LEFT JOIN libraries l ON b.library_id = l.library_id
		WHERE books_fts MATCH ? AND b.deleted = 0 `)

	// Language filter condition
	langCondition := ""
	if len(languages) > 0 {
		// Treat empty language as "ru"
		// If "ru" is requested, also include "" (empty string) in the loop/IN clause
		searchLangs := make([]string, 0, len(languages)+1)
		for _, l := range languages {
			searchLangs = append(searchLangs, l)
			if strings.EqualFold(l, "ru") {
				searchLangs = append(searchLangs, "")
			}
		}

		lArgs, placeholders := buildSliceArgs(searchLangs)
		langCondition = fmt.Sprintf("AND b.lang IN (%s)", placeholders)
		args = append(args, lArgs...)
	}
	queryBuilder.WriteString(" ")
	queryBuilder.WriteString(langCondition)

	// Sort by the author names: strip the "id<char(31)>" prefix so the order
	// matches the plain name string the query used to sort by.
	queryBuilder.WriteString(`
		GROUP BY b.book_id, b.title, b.lang, b.archive, b.filename, b.file_size, b.deleted, s.name, bs.series_no, bs.series_id, fts.rank, aa.author_combined, l.name, COALESCE(l.display_name, l.name)
		ORDER BY substr(aa.author_combined, instr(aa.author_combined, char(31)) + 1), s.name, bs.series_no, b.title COLLATE NOCASE
		LIMIT ? OFFSET ?
	`)

	QUERY := queryBuilder.String()
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, QUERY, args...)
	if err != nil {
		return nil, fmt.Errorf("search books: %w", err)
	}
	defer rows.Close()

	results := make([]book.BookSearchResult, 0)
	for rows.Next() {
		var r book.BookSearchResult
		var seriesName sql.NullString
		var seriesNo sql.NullInt64
		var seriesID sql.NullInt64
		var genresStr sql.NullString
		var authorStr sql.NullString
		var libraryName, libraryDisplayName sql.NullString

		err := rows.Scan(
			&r.BookID, &r.Title, &r.Lang, &r.Archive, &r.FileName,
			&r.FileSize, &r.Deleted, &seriesName, &seriesNo, &seriesID,
			&r.Rank, &authorStr, &genresStr, &libraryName, &libraryDisplayName,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}

		if authorStr.Valid {
			r.Author, r.AuthorIDs = splitAuthorCombined(authorStr.String)
		}
		if seriesName.Valid {
			r.SeriesName = seriesName.String
		}
		if seriesNo.Valid {
			r.SeriesNo = int(seriesNo.Int64)
		}
		if seriesID.Valid {
			r.SeriesID = seriesID.Int64
		}
		if genresStr.Valid {
			r.Genres = strings.Split(genresStr.String, ",")
		}
		r.Library = libraryName.String
		r.LibraryDisplayName = libraryDisplayName.String

		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	return results, nil
}

// splitAuthorCombined decodes the pre-aggregated author string produced by
// the SearchBooks derived table: "id<char(31)>name" entries joined by
// char(30). It returns the visible author string (names joined with ",",
// byte-identical to the plain group_concat the query used to produce) and the
// author IDs in the same order. Entries without the separator or with a
// non-numeric ID cannot be produced by the writer and are skipped.
func splitAuthorCombined(combined string) (string, []int64) {
	entries := strings.Split(combined, "\x1e") // char(30)
	ids := make([]int64, 0, len(entries))
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		sep := strings.Index(entry, "\x1f") // char(31)
		if sep < 0 {
			continue
		}
		id, err := strconv.ParseInt(entry[:sep], 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
		names = append(names, entry[sep+1:])
	}
	return strings.Join(names, ","), ids
}

// RebuildFTSIndex rebuilds the full-text search index for all books
// Updates the author field in books_fts to include properly concatenated author names
func (r *Repo) RebuildFTSIndex() error {
	// Rebuild FTS index from scratch
	// 1. Clear FTS table
	if _, err := r.db.Exec("DELETE FROM books_fts"); err != nil {
		return fmt.Errorf("rebuild FTS index (delete): %w", err)
	}

	// 2. Insert all books with their metadata
	QUERY := `
		INSERT INTO books_fts(title, author, series, genre, book_id)
		SELECT
			b.title,
			(SELECT group_concat(a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, ''), ' | ')
			 FROM book_authors ba
			 JOIN authors a ON ba.author_id = a.author_id
			 WHERE ba.book_id = b.book_id),
			(SELECT s.name
			 FROM book_series bs
			 JOIN series s ON bs.series_id = s.series_id
			 WHERE bs.book_id = b.book_id),
			(SELECT group_concat(g.name || ' ' || coalesce(g.display_name, '') || ' ' || coalesce(g.translit_name, ''), ' | ')
			 FROM book_genres bg
			 JOIN genres g ON bg.genre_id = g.genre_id
			 WHERE bg.book_id = b.book_id),
			b.book_id
		FROM books b
		WHERE b.deleted = 0
	`
	result, err := r.db.Exec(QUERY)
	if err != nil {
		return fmt.Errorf("rebuild FTS index (insert): %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Info("FTS index rebuilt", "rows_updated", rowsAffected)

	return nil
}

// buildSliceArgs generates placeholders and converts slice to []interface{}
// e.g. buildSliceArgs([]string{"a", "b"}) -> ([]interface{}{"a", "b"}, "?,?")
func buildSliceArgs(items []string) ([]interface{}, string) {
	if len(items) == 0 {
		return nil, ""
	}
	args := make([]interface{}, len(items))
	placeholders := make([]string, len(items))
	for i, item := range items {
		args[i] = item
		placeholders[i] = "?"
	}
	return args, strings.Join(placeholders, ",")
}
