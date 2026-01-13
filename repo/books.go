package repo

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/config"
	"github.com/htol/bopds/logger"
	_ "github.com/mattn/go-sqlite3"
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

type Repo struct {
	db   *sql.DB
	path string
}

var _ Repository = (*Repo)(nil)

func GetStorage(path string) *Repo {
	return GetStorageWithConfig(path, config.Load())
}

func GetStorageWithConfig(path string, cfg *config.Config) *Repo {
	r := &Repo{
		path: path,
	}

	db, err := sql.Open("sqlite3", "file:"+r.path+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		logger.Error("Failed to open database", "path", r.path, "error", err)
		panic(err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	r.db = db

	// TODO: Drop indexes
	sqlStmt := `
           CREATE TABLE IF NOT EXISTS "authors" (
               author_id integer primary key autoincrement not null,
               first_name text,
               middle_name text,
               last_name text,
               UNIQUE(first_name, middle_name, last_name)
           );
           CREATE INDEX IF NOT EXISTS [I_first_name] ON "authors" ([first_name]);
           CREATE INDEX IF NOT EXISTS [I_last_name] ON "authors" ([last_name]);
           CREATE INDEX IF NOT EXISTS [I_middle_name] ON "authors" ([middle_name]);

           CREATE TABLE IF NOT EXISTS "books" (
                book_id integer primary key autoincrement not null,
                title text,
                lang text,
                archive text,
                filename text,
                file_size integer,
                date_added text,
                lib_id integer,
                deleted boolean default 0, -- 0=present/active, 1=marked for deletion or absent
                lib_rate integer
            );
           CREATE INDEX IF NOT EXISTS [I_title] ON "books" ([title]);

           CREATE TABLE IF NOT EXISTS "book_authors" (
               book_id INTEGER NOT NULL,
               author_id INTEGER NOT NULL,
               FOREIGN KEY (book_id) REFERENCES books(book_id),
               FOREIGN KEY (author_id) REFERENCES authors(author_id)
           );
           CREATE INDEX IF NOT EXISTS [I_book_id] ON "book_authors" ([book_id]);
           CREATE INDEX IF NOT EXISTS [I_author_id] ON "book_authors" ([author_id]);

           CREATE TABLE IF NOT EXISTS "genres" (
               genre_id integer primary key autoincrement not null,
               name text unique not null
           );

           CREATE TABLE IF NOT EXISTS "book_genres" (
               book_id INTEGER NOT NULL,
               genre_id INTEGER NOT NULL,
               PRIMARY KEY (book_id, genre_id),
               FOREIGN KEY (book_id) REFERENCES books(book_id),
               FOREIGN KEY (genre_id) REFERENCES genres(genre_id)
           );

           CREATE TABLE IF NOT EXISTS "series" (
               series_id INTEGER PRIMARY KEY AUTOINCREMENT,
               name TEXT UNIQUE NOT NULL
           );

           CREATE TABLE IF NOT EXISTS "book_series" (
               book_id INTEGER NOT NULL,
               series_id INTEGER NOT NULL,
               series_no INTEGER,
               PRIMARY KEY (book_id, series_id),
               FOREIGN KEY (book_id) REFERENCES books(book_id) ON DELETE CASCADE,
               FOREIGN KEY (series_id) REFERENCES series(series_id)
           );
           CREATE INDEX IF NOT EXISTS [idx_book_series_book_id] ON [book_series] ([book_id]);
           CREATE INDEX IF NOT EXISTS [idx_book_series_series_id] ON [book_series] ([series_id]);

           CREATE TABLE IF NOT EXISTS "keywords" (
               keyword_id INTEGER PRIMARY KEY AUTOINCREMENT,
               name TEXT UNIQUE NOT NULL
           );

           CREATE TABLE IF NOT EXISTS "book_keywords" (
               book_id INTEGER NOT NULL,
               keyword_id INTEGER NOT NULL,
               PRIMARY KEY (book_id, keyword_id),
               FOREIGN KEY (book_id) REFERENCES books(book_id) ON DELETE CASCADE,
               FOREIGN KEY (keyword_id) REFERENCES keywords(keyword_id)
           );
           CREATE INDEX IF NOT EXISTS [idx_book_keywords_book_id] ON [book_keywords] ([book_id]);
           CREATE INDEX IF NOT EXISTS [idx_book_keywords_keyword_id] ON [book_keywords] ([keyword_id]);

           CREATE VIRTUAL TABLE IF NOT EXISTS books_fts USING fts5(title, author, series, book_id);
  	    `

	_, err = db.Exec(sqlStmt)
	if err != nil {
		logger.Error("Failed to execute schema", "error", err)
		panic(err)
	}

	// Recreate triggers to ensure they are up-to-date
	triggerStmt := `
           DROP TRIGGER IF EXISTS books_fts_insert;
           CREATE TRIGGER books_fts_insert AFTER INSERT ON books BEGIN
               INSERT INTO books_fts(title, author, series, book_id)
               VALUES (
                 new.title,
                 (SELECT group_concat(a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, ''), ' | ')
                  FROM book_authors ba
                  LEFT JOIN authors a ON ba.author_id = a.author_id
                  WHERE ba.book_id = new.book_id),
                 (SELECT s.name FROM book_series bs LEFT JOIN series s ON bs.series_id = s.series_id WHERE bs.book_id = new.book_id),
                 new.book_id
               );
            END;

           DROP TRIGGER IF EXISTS books_fts_delete;
           CREATE TRIGGER books_fts_delete AFTER DELETE ON books BEGIN
               DELETE FROM books_fts WHERE book_id = old.book_id;
           END;

           DROP TRIGGER IF EXISTS books_fts_authors_insert;
           CREATE TRIGGER books_fts_authors_insert AFTER INSERT ON book_authors BEGIN
                UPDATE books_fts
                SET author = (
                  SELECT group_concat(a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, ''), ' | ')
                  FROM book_authors ba
                  LEFT JOIN authors a ON ba.author_id = a.author_id
                  WHERE ba.book_id = new.book_id
                )
                WHERE book_id = new.book_id;
           END;

           DROP TRIGGER IF EXISTS books_fts_authors_delete;
              CREATE TRIGGER books_fts_authors_delete AFTER DELETE ON book_authors BEGIN
                UPDATE books_fts
                SET author = (
                  SELECT group_concat(a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, ''), ' | ')
                  FROM book_authors ba
                  LEFT JOIN authors a ON ba.author_id = a.author_id
                  WHERE ba.book_id = old.book_id
                )
                WHERE book_id = old.book_id;
              END;

           DROP TRIGGER IF EXISTS books_fts_series_insert;
           CREATE TRIGGER books_fts_series_insert AFTER INSERT ON book_series BEGIN
                UPDATE books_fts
                SET series = (
                  SELECT s.name 
                  FROM book_series bs 
                  JOIN series s ON bs.series_id = s.series_id 
                  WHERE bs.book_id = new.book_id
                )
                WHERE book_id = new.book_id;
           END;

           DROP TRIGGER IF EXISTS books_fts_series_delete;
           CREATE TRIGGER books_fts_series_delete AFTER DELETE ON book_series BEGIN
                UPDATE books_fts
                SET series = NULL
                WHERE book_id = old.book_id;
           END;
    `
	_, err = db.Exec(triggerStmt)
	if err != nil {
		logger.Error("Failed to update triggers", "error", err)
		panic(err)
	}

	return r
}

func (r *Repo) Close() error {
	return r.db.Close()
}

// Ping checks if database connection is alive
func (r *Repo) Ping() error {
	return r.db.Ping()
}

func (r *Repo) Add(record *book.Book) error {
	authorsIDs, err := r.getOrCreateAuthor(record.Author)
	if err != nil {
		return fmt.Errorf("get or create author: %w", err)
	}

	insertBook := `
		INSERT INTO books(title, lang, archive, filename,
				file_size, date_added, lib_id, deleted, lib_rate)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	insertStm, err := r.db.Prepare(insertBook)
	if err != nil {
		return fmt.Errorf("prepare insert book: %w", err)
	}
	defer insertStm.Close()

	deletedInt := 0
	if record.Deleted {
		deletedInt = 1
	}

	sqlresult, err := insertStm.Exec(
		record.Title,
		record.Lang,
		record.Archive,
		record.FileName,
		record.FileSize,
		record.DateAdded,
		record.LibID,
		deletedInt,
		record.LibRate,
	)
	if err != nil {
		return fmt.Errorf("insert book: %w", err)
	}
	bookID, err := sqlresult.LastInsertId()
	if err != nil {
		return fmt.Errorf("get book id: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := r.linkAuthors(tx, bookID, authorsIDs); err != nil {
		tx.Rollback()
		return err
	}

	if len(record.Genres) > 0 {
		if err := r.linkGenres(tx, bookID, record.Genres); err != nil {
			tx.Rollback()
			return err
		}
	}

	if record.Series != nil && record.Series.Name != "" {
		if err := r.linkSeries(tx, bookID, record.Series); err != nil {
			tx.Rollback()
			return err
		}
	}

	if len(record.Keywords) > 0 {
		if err := r.linkKeywords(tx, bookID, record.Keywords); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *Repo) Search() error {
	return nil
}

func (r *Repo) List() error {
	return nil
}

func (r *Repo) linkAuthors(tx *sql.Tx, bookID int64, authorIDs []int64) error {
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO book_authors(book_id, author_id) VALUES(?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare book_authors: %w", err)
	}
	defer stmt.Close()

	for _, authorID := range authorIDs {
		if _, err := stmt.Exec(bookID, authorID); err != nil {
			return fmt.Errorf("insert book_author: %w", err)
		}
	}
	return nil
}

// Link series to book
func (r *Repo) linkSeries(tx *sql.Tx, bookID int64, series *book.SeriesInfo) error {
	seriesID, err := r.getOrCreateSeries(tx, series.Name)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO book_series (book_id, series_id, series_no)
		VALUES (?, ?, ?)
	`, bookID, seriesID, series.SeriesNo)

	return err
}

func (r *Repo) linkKeywords(tx *sql.Tx, bookID int64, keywords []string) error {
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}

		keywordID, err := r.getOrCreateKeyword(tx, keyword)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT OR IGNORE INTO book_keywords (book_id, keyword_id)
			VALUES (?, ?)
		`, bookID, keywordID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) linkGenres(tx *sql.Tx, bookID int64, genres []string) error {
	if len(genres) == 0 {
		return nil
	}

	getGenreStmt, err := tx.Prepare(`SELECT genre_id FROM genres WHERE name = ?`)
	if err != nil {
		return fmt.Errorf("prepare get genre: %w", err)
	}
	defer getGenreStmt.Close()

	insertGenreStmt, err := tx.Prepare(`INSERT INTO genres(name) VALUES(?)`)
	if err != nil {
		return fmt.Errorf("prepare insert genre: %w", err)
	}
	defer insertGenreStmt.Close()

	bookGenreStmt, err := tx.Prepare(`INSERT OR IGNORE INTO book_genres(book_id, genre_id) VALUES(?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare book_genre: %w", err)
	}
	defer bookGenreStmt.Close()

	for _, genre := range genres {
		if genre == "" {
			continue
		}

		var genreID int64
		err := getGenreStmt.QueryRow(genre).Scan(&genreID)
		if err == sql.ErrNoRows {
			result, err := insertGenreStmt.Exec(genre)
			if err != nil {
				return fmt.Errorf("insert genre: %w", err)
			}
			genreID, err = result.LastInsertId()
			if err != nil {
				return fmt.Errorf("get genre id: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("get genre id: %w", err)
		}

		if _, err := bookGenreStmt.Exec(bookID, genreID); err != nil {
			return fmt.Errorf("insert book_genre: %w", err)
		}
	}

	return nil
}

// Get or create series
func (r *Repo) getOrCreateSeries(tx *sql.Tx, name string) (int64, error) {
	var seriesID int64
	err := tx.QueryRow(`SELECT series_id FROM series WHERE name = ?`, name).Scan(&seriesID)
	if err == sql.ErrNoRows {
		result, err := tx.Exec(`INSERT INTO series(name) VALUES(?)`, name)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	}
	return seriesID, err
}

func (r *Repo) getOrCreateKeyword(tx *sql.Tx, name string) (int64, error) {
	var keywordID int64
	err := tx.QueryRow(`SELECT keyword_id FROM keywords WHERE name = ?`, name).Scan(&keywordID)
	if err == sql.ErrNoRows {
		result, err := tx.Exec(`INSERT INTO keywords(name) VALUES(?)`, name)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	}
	return keywordID, err
}

func (r *Repo) getOrCreateAuthor(authors []book.Author) ([]int64, error) {
	if len(authors) < 1 {
		return nil, nil
	}

	result := []int64{}

	selectStmt, err := r.db.Prepare(`
		SELECT author_id FROM authors
		WHERE first_name = ? AND middle_name = ? AND last_name = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare select author: %w", err)
	}
	defer selectStmt.Close()

	insertStm, err := r.db.Prepare(`
		INSERT INTO authors(first_name, middle_name, last_name) VALUES(?, ?, ?) RETURNING author_id
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert author: %w", err)
	}
	defer insertStm.Close()

	for _, author := range authors {
		var authorID int64

		err := selectStmt.QueryRow(
			author.FirstName,
			author.MiddleName,
			author.LastName,
		).Scan(&authorID)

		if err == sql.ErrNoRows {
			err := insertStm.QueryRow(
				author.FirstName,
				author.MiddleName,
				author.LastName,
			).Scan(&authorID)
			if err != nil {
				return nil, fmt.Errorf("insert author: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("select author: %w", err)
		}

		result = append(result, authorID)
	}
	return result, nil
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
	pattern := strings.Title(letters) + "%"
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
	pattern := strings.Title(letters) + "%"
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
	pattern := strings.Title(letters) + "%"
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
			// Books with series usually come together.
			// If one has series and other doesn't (and authors match),
			// usually we want to group series.
			// Plain string comparison works: empty string < non-empty.
			// So books without series come first? Or last?
			// Let's stick to standard string compare for now.
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

func (r *Repo) GetGenres() ([]string, error) {
	QUERY := `
		SELECT DISTINCT g.name 
		FROM genres g
		JOIN book_genres bg ON g.genre_id = bg.genre_id
		JOIN books b ON bg.book_id = b.book_id
		WHERE b.deleted = 0
		ORDER BY g.name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query genres: %w", err)
	}
	defer rows.Close()

	genres := make([]string, 0)
	for rows.Next() {
		var genre string
		if err := rows.Scan(&genre); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		genres = append(genres, genre)
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

// SearchBooks performs full-text search across book titles and authors
// Uses FTS5 for fast, ranked search results
// Optimized with single query including author JOIN (fixes N+1 query issue)
func (r *Repo) SearchBooks(ctx context.Context, query string, limit, offset int) ([]book.BookSearchResult, error) {
	// Validate query
	if query == "" {
		return []book.BookSearchResult{}, nil
	}

	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return []book.BookSearchResult{}, nil
	}

	// Escape FTS5 special characters to prevent injection
	ftsQuery := escapeFTS5Query(cleanQuery) + "*"

	// Search FTS5 table and join back to books table for full details
	// Uses book_id column for direct, accurate mapping
	QUERY := `
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
			fts.rank,
			group_concat(distinct a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '')) as author
		FROM books_fts fts
		JOIN books b ON fts.book_id = b.book_id
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE books_fts MATCH ? AND b.deleted = 0
		GROUP BY b.book_id, b.title, b.lang, b.archive, b.filename, b.file_size, b.deleted, s.name, bs.series_no, fts.rank
		ORDER BY author, s.name, bs.series_no, b.title COLLATE NOCASE
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, QUERY, ftsQuery, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search books: %w", err)
	}
	defer rows.Close()

	results := make([]book.BookSearchResult, 0)
	for rows.Next() {
		var r book.BookSearchResult
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		err := rows.Scan(
			&r.BookID, &r.Title, &r.Lang, &r.Archive, &r.FileName,
			&r.FileSize, &r.Deleted, &seriesName, &seriesNo,
			&r.Rank, &r.Author,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}

		if seriesName.Valid {
			r.SeriesName = seriesName.String
		}
		if seriesNo.Valid {
			r.SeriesNo = int(seriesNo.Int64)
		}

		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	return results, nil
}

// RebuildFTSIndex rebuilds the full-text search index for all books
// Updates the author field in books_fts to include properly concatenated author names
func (r *Repo) RebuildFTSIndex() error {
	// Update authors
	UPDATE_AUTHORS := `
		UPDATE books_fts
		SET author = (
			SELECT group_concat(a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, ''), ' | ')
			FROM book_authors ba
			LEFT JOIN authors a ON ba.author_id = a.author_id
			WHERE ba.book_id = books_fts.book_id
		)
	`
	if _, err := r.db.Exec(UPDATE_AUTHORS); err != nil {
		return fmt.Errorf("rebuild FTS index (authors): %w", err)
	}

	// Update series
	UPDATE_SERIES := `
		UPDATE books_fts
		SET series = (
			SELECT s.name 
            FROM book_series bs 
            JOIN series s ON bs.series_id = s.series_id 
            WHERE bs.book_id = books_fts.book_id
		)
	`
	result, err := r.db.Exec(UPDATE_SERIES)
	if err != nil {
		return fmt.Errorf("rebuild FTS index (series): %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Info("FTS index rebuilt", "rows_updated", rowsAffected)

	return nil
}
