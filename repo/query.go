package repo

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// sortBooks sorts books by Author -> Series -> SeriesNo -> Title
func sortBooks(books []book.Book) {
	sort.Slice(books, func(i, j int) bool {
		b1 := &books[i]
		b2 := &books[j]

		// 1. Author (Last Name, First Name) - First author only
		var a1, a2 book.Author
		if len(b1.Author) > 0 {
			a1 = b1.Author[0]
		}
		if len(b2.Author) > 0 {
			a2 = b2.Author[0]
		}

		if a1.LastName != a2.LastName {
			return strings.ToLower(a1.LastName) < strings.ToLower(a2.LastName)
		}
		if a1.FirstName != a2.FirstName {
			return strings.ToLower(a1.FirstName) < strings.ToLower(a2.FirstName)
		}

		// 2. Series Name
		var s1, s2 string
		if b1.Series != nil {
			s1 = b1.Series.Name
		}
		if b2.Series != nil {
			s2 = b2.Series.Name
		}
		if s1 != s2 {
			return strings.ToLower(s1) < strings.ToLower(s2)
		}

		// 3. Series Number
		var sn1, sn2 int
		if b1.Series != nil {
			sn1 = b1.Series.SeriesNo
		}
		if b2.Series != nil {
			sn2 = b2.Series.SeriesNo
		}
		if sn1 != sn2 {
			return sn1 < sn2
		}

		// 4. Title
		return strings.ToLower(b1.Title) < strings.ToLower(b2.Title)
	})
}

func (r *Repo) GetAuthors() ([]book.Author, error) {
	QUERY := `
		SELECT DISTINCT a.author_id, a.first_name, a.middle_name, a.last_name
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE b.deleted = 0
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query authors: %w", err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author

		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return nil, fmt.Errorf("scan author: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorsByLetter(letters string) ([]book.Author, error) {
	pattern := cases.Title(language.Und, cases.NoLower).String(letters) + "%"
	QUERY := `
		SELECT DISTINCT a.author_id, a.first_name, a.middle_name, a.last_name
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE a.last_name LIKE ? COLLATE NOCASE
		AND b.deleted = 0
		ORDER BY a.last_name
	`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query authors by letter: %w", err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author

		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return nil, fmt.Errorf("scan author by letter: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors by letter: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorsWithBookCount() ([]book.AuthorWithBookCount, error) {
	QUERY := `
		SELECT a.author_id, a.first_name, a.middle_name, a.last_name,
			   COUNT(b.book_id) as book_count
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE b.deleted = 0
		GROUP BY a.author_id, a.first_name, a.middle_name, a.last_name
		ORDER BY a.last_name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query authors with book count: %w", err)
	}
	defer rows.Close()

	authors := make([]book.AuthorWithBookCount, 0)
	for rows.Next() {
		var a book.AuthorWithBookCount
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.BookCount); err != nil {
			return nil, fmt.Errorf("scan author with count: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors with count: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorsWithBookCountByLetter(letters string) ([]book.AuthorWithBookCount, error) {
	pattern := cases.Title(language.Und, cases.NoLower).String(letters) + "%"
	QUERY := `
		SELECT a.author_id, a.first_name, a.middle_name, a.last_name,
			   COUNT(b.book_id) as book_count
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE a.last_name LIKE ? COLLATE NOCASE
		AND b.deleted = 0
		GROUP BY a.author_id, a.first_name, a.middle_name, a.last_name
		ORDER BY a.last_name
	`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query authors with book count by letter: %w", err)
	}
	defer rows.Close()

	authors := make([]book.AuthorWithBookCount, 0)
	for rows.Next() {
		var a book.AuthorWithBookCount
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.BookCount); err != nil {
			return nil, fmt.Errorf("scan author with count by letter: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors with count by letter: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorByID(id int64) (*book.Author, error) {
	QUERY := `SELECT author_id, first_name, middle_name, last_name FROM authors WHERE author_id = ?`

	var a book.Author
	err := r.db.QueryRow(QUERY, id).Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get author by ID %d: %w", id, err)
	}

	return &a, nil
}

func (r *Repo) GetBooks() ([]string, error) {
	QUERY := `SELECT * FROM books WHERE deleted = 0`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query books: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get book columns: %w", err)
	}

	books := make([]string, 0)

	for rows.Next() {
		columns := make([]sql.NullString, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, fmt.Errorf("scan book: %w", err)
		}
		var sb strings.Builder
		for i := range cols {
			fmt.Fprintf(&sb, "%s, ", columns[i].String)
		}

		books = append(books, sb.String())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books: %w", err)
	}

	return books, nil
}

func (r *Repo) GetBooksByLetter(letters string) ([]book.Book, error) {
	pattern := cases.Title(language.Und, cases.NoLower).String(letters) + "%"
	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE b.title LIKE ? COLLATE NOCASE AND b.deleted = 0
		ORDER BY b.title
	`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query books by letter: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, fmt.Errorf("scan book by letter: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		// Helper to construct series info
		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			// Check if author is new
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}

			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}

			// If series is missing, add it
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}
		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by letter: %w", err)
	}

	// Re-sort in Go to be fast and correct (since map randomization)
	books := make([]book.Book, 0, len(booksMap))
	for _, book := range booksMap {
		books = append(books, *book)
	}

	sortBooks(books)

	return books, nil
}

func (r *Repo) GetBooksByAuthorID(id int64) ([]book.Book, error) {
	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE ba.author_id = ? AND b.deleted = 0
		ORDER BY b.title
	`

	rows, err := r.db.Query(QUERY, id)
	if err != nil {
		return nil, fmt.Errorf("query books by author id: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, fmt.Errorf("scan book by author id: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		// Helper to construct series info
		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			// Check if author is new (avoid duplicates if multiple series rows caused duplication)
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}

			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}

			// If series is missing, add it
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}

		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by author id: %w", err)
	}

	// Re-sort using universal logic
	books := make([]book.Book, 0, len(booksMap))
	for _, book := range booksMap {
		books = append(books, *book)
	}

	sortBooks(books)

	return books, nil
}

func (r *Repo) GetGenres() ([]book.Genre, error) {
	QUERY := `
		SELECT g.genre_id, g.name, g.display_name
		FROM genres g
		JOIN book_genres bg ON g.genre_id = bg.genre_id
		JOIN books b ON bg.book_id = b.book_id
		WHERE b.deleted = 0
		GROUP BY g.genre_id
		ORDER BY g.display_name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query genres: %w", err)
	}
	defer rows.Close()

	genres := make([]book.Genre, 0)
	for rows.Next() {
		var g book.Genre
		var displayName sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &displayName); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		if displayName.Valid {
			g.DisplayName = displayName.String
		} else {
			g.DisplayName = g.Name // Fallback
		}
		genres = append(genres, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate genres: %w", err)
	}

	return genres, nil
}

func (r *Repo) GetBookByID(id int64) (*book.Book, error) {
	QUERY := `
		SELECT book_id, title, lang, archive, filename,
			   file_size, date_added, lib_id, deleted, lib_rate
		FROM books
		WHERE book_id = ? AND deleted = 0
	`

	var b book.Book
	var deleted bool
	var libRate sql.NullInt64

	err := r.db.QueryRow(QUERY, id).Scan(
		&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
		&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get book by ID %d: %w", id, err)
	}

	b.Deleted = deleted
	if libRate.Valid {
		b.LibRate = int(libRate.Int64)
	}

	// Fetch related data
	if err := r.fetchBookDetails(&b); err != nil {
		return nil, err
	}

	return &b, nil
}

// GetRecentBooks returns recently added books with pagination
func (r *Repo) GetRecentBooks(limit, offset int) ([]book.Book, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM books WHERE deleted = 0`
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count recent books: %w", err)
	}

	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE b.deleted = 0
		ORDER BY b.date_added DESC, b.book_id DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(QUERY, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query recent books: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	bookOrder := make([]int64, 0)

	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, 0, fmt.Errorf("scan recent book: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}
			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}
		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
			bookOrder = append(bookOrder, b.BookID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate recent books: %w", err)
	}

	// Preserve order (most recent first)
	books := make([]book.Book, 0, len(bookOrder))
	for _, id := range bookOrder {
		books = append(books, *booksMap[id])
	}

	return books, total, nil
}

// GetBooksByGenre returns books by genre with pagination
func (r *Repo) GetBooksByGenre(genre string, limit, offset int) ([]book.Book, int, error) {
	// Get total count for this genre
	countQuery := `
		SELECT COUNT(DISTINCT b.book_id)
		FROM books b
		JOIN book_genres bg ON b.book_id = bg.book_id
		JOIN genres g ON bg.genre_id = g.genre_id
		WHERE (g.display_name = ? OR g.name = ?) AND b.deleted = 0
	`
	var total int
	if err := r.db.QueryRow(countQuery, genre, genre).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count books by genre: %w", err)
	}

	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		JOIN book_genres bg ON b.book_id = bg.book_id
		JOIN genres g ON bg.genre_id = g.genre_id
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE (g.display_name = ? OR g.name = ?) AND b.deleted = 0
		ORDER BY b.title
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(QUERY, genre, genre, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query books by genre: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	bookOrder := make([]int64, 0)

	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, 0, fmt.Errorf("scan book by genre: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}
			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}
		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
			bookOrder = append(bookOrder, b.BookID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate books by genre: %w", err)
	}

	// Preserve order
	books := make([]book.Book, 0, len(bookOrder))
	for _, id := range bookOrder {
		books = append(books, *booksMap[id])
	}

	sortBooks(books)

	return books, total, nil
}

// NEW: Fetch all related details for a book
func (r *Repo) fetchBookDetails(b *book.Book) error {
	// Fetch authors
	authorsQuery := `
		SELECT a.author_id, a.first_name, a.middle_name, a.last_name
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		WHERE ba.book_id = ?
		ORDER BY a.last_name, a.first_name
	`

	rows, err := r.db.Query(authorsQuery, b.BookID)
	if err != nil {
		return fmt.Errorf("query authors for book %d: %w", b.BookID, err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return fmt.Errorf("scan author for book %d: %w", b.BookID, err)
		}
		authors = append(authors, a)
	}
	b.Author = authors

	seriesQuery := `
		SELECT s.series_id, s.name, bs.series_no
		FROM series s
		JOIN book_series bs ON s.series_id = bs.series_id
		WHERE bs.book_id = ?
	`

	var seriesID int64
	var seriesName sql.NullString
	var seriesNo sql.NullInt64

	err = r.db.QueryRow(seriesQuery, b.BookID).Scan(&seriesID, &seriesName, &seriesNo)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("query series for book %d: %w", b.BookID, err)
	}

	if seriesName.Valid {
		b.Series = &book.SeriesInfo{
			ID:       seriesID,
			Name:     seriesName.String,
			SeriesNo: int(seriesNo.Int64),
		}
	}

	keywordsQuery := `
		SELECT k.keyword_id, k.name
		FROM keywords k
		JOIN book_keywords bk ON k.keyword_id = bk.keyword_id
		WHERE bk.book_id = ?
		ORDER BY k.name
	`

	rows, err = r.db.Query(keywordsQuery, b.BookID)
	if err != nil {
		return fmt.Errorf("query keywords for book %d: %w", b.BookID, err)
	}
	defer rows.Close()

	keywords := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(new(int64), &name); err != nil {
			return err
		}
		keywords = append(keywords, name)
	}
	b.Keywords = keywords

	return nil
}

// GetSeries Get all series
func (r *Repo) GetSeries() ([]book.SeriesInfo, error) {
	QUERY := `
		SELECT DISTINCT s.series_id, s.name
		FROM series s
		JOIN book_series bs ON s.series_id = bs.series_id
		JOIN books b ON bs.book_id = b.book_id
		WHERE b.deleted = 0
		ORDER BY s.name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query series: %w", err)
	}
	defer rows.Close()

	series := make([]book.SeriesInfo, 0)
	for rows.Next() {
		var s book.SeriesInfo
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, fmt.Errorf("scan series: %w", err)
		}
		series = append(series, s)
	}

	return series, nil
}

// GetBooksBySeriesID books by series
func (r *Repo) GetBooksBySeriesID(seriesID int64) ([]book.Book, error) {
	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate
		FROM books b
		JOIN book_series bs ON b.book_id = bs.book_id
		WHERE bs.series_id = ? AND b.deleted = 0
		ORDER BY bs.series_no, b.title
	`

	rows, err := r.db.Query(QUERY, seriesID)
	if err != nil {
		return nil, fmt.Errorf("query books by series: %w", err)
	}
	defer rows.Close()

	books := make([]book.Book, 0)
	for rows.Next() {
		var b book.Book
		var deleted bool
		var libRate sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
		); err != nil {
			return nil, fmt.Errorf("scan book: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		books = append(books, b)
	}

	sortBooks(books)

	return books, nil
}

// GetKeywords Get all keywords
func (r *Repo) GetKeywords() ([]book.Keyword, error) {
	QUERY := `
		SELECT DISTINCT k.keyword_id, k.name
		FROM keywords k
		JOIN book_keywords bk ON k.keyword_id = bk.keyword_id
		JOIN books b ON bk.book_id = b.book_id
		WHERE b.deleted = 0
		ORDER BY k.name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query keywords: %w", err)
	}
	defer rows.Close()

	keywords := make([]book.Keyword, 0)
	for rows.Next() {
		var k book.Keyword
		if err := rows.Scan(&k.ID, &k.Name); err != nil {
			return nil, fmt.Errorf("scan keyword: %w", err)
		}
		keywords = append(keywords, k)
	}

	return keywords, nil
}

func (r *Repo) SyncGenreDisplayNames() {
	rows, err := r.db.Query(`SELECT name, display_name, translit_name FROM genres`)
	if err != nil {
		logger.Error("Failed to query genres for update", "error", err)
		return
	}

	type genreUpdate struct {
		name         string
		displayName  string
		translitName string
	}
	var updates []genreUpdate

	for rows.Next() {
		var name string
		var currentDN sql.NullString
		var currentTN sql.NullString
		if err := rows.Scan(&name, &currentDN, &currentTN); err == nil {
			displayName := MapGenre(name)
			translitName := Translit(displayName)

			needsUpdate := false
			if displayName != name && (!currentDN.Valid || currentDN.String != displayName) {
				needsUpdate = true
			}
			if !currentTN.Valid || currentTN.String != translitName {
				needsUpdate = true
			}

			if needsUpdate {
				updates = append(updates, genreUpdate{name, displayName, translitName})
			}
		}
	}
	rows.Close()

	if len(updates) == 0 {
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		logger.Error("Failed to begin transaction for genre update", "error", err)
		return
	}

	stmt, err := tx.Prepare(`UPDATE genres SET display_name = ?, translit_name = ? WHERE name = ?`)
	if err != nil {
		logger.Error("Failed to prepare statement for genre update", "error", err)
		if err := tx.Rollback(); err != nil {
			logger.Error("Failed to rollback transaction", "error", err)
		}
		return
	}
	defer stmt.Close()

	count := 0
	for _, u := range updates {
		if _, err := stmt.Exec(u.displayName, u.translitName, u.name); err != nil {
			logger.Error("Failed to execute genre update", "error", err, "name", u.name)
		} else {
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Error("Failed to commit genre updates", "error", err)
	} else if count > 0 {
		logger.Info("Updated genre display/translit names", "new_count", count)
	}
}

// GetLanguages returns a list of distinct languages from non-deleted books
func (r *Repo) GetLanguages() ([]string, error) {
	// Treat empty or null language as 'ru'
	QUERY := `
		SELECT DISTINCT 
			CASE WHEN IFNULL(lang, '') = '' THEN 'ru' ELSE lang END as language 
		FROM books 
		WHERE deleted = 0 
		ORDER BY language
	`
	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("get languages: %w", err)
	}
	defer rows.Close()

	var languages []string
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			return nil, fmt.Errorf("scan language: %w", err)
		}
		languages = append(languages, lang)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate languages: %w", err)
	}
	return languages, nil
}
