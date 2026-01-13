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

	// Performance optimizations
	// 1. Memory Mapped I/O: Map up to 30GB. If file is smaller, maps entire file.
	if _, err := db.Exec("PRAGMA mmap_size = 30000000000"); err != nil {
		logger.Warn("Failed to set mmap_size", "error", err)
	}

	// 2. Cache Size: -64000 means 64MB of cache. Positive would be N pages.
	// 64MB is a safe conservative default.
	if _, err := db.Exec("PRAGMA cache_size = -64000"); err != nil {
		logger.Warn("Failed to set cache_size", "error", err)
	}

	// 3. Temporary Store: Use RAM for temp tables/indices (critical for sorting during CREATE INDEX)
	if _, err := db.Exec("PRAGMA temp_store = MEMORY"); err != nil {
		logger.Warn("Failed to set temp_store", "error", err)
	}

	r.db = db

	// Create tables first (without indexes for authors/books to allow fast bulk insert option)
	// We still need uniqueness constraints on core tables if we rely on them for logic,
	// but pure performance indexes can be managed separately.
	// Actually, for safety, let's keep the CREATE TABLE + UNIQUE constraints in CreateIndexes or here?
	// The problem is if we drop indexes that back UNIQUE constraints, we lose the constraint.
	// SQLite: "Books" table doesn't have unique constraints other than PK.
	// "Authors" has UNIQUE(first, middle, last). If we drop that, we might get duplicates if logic fails.
	// However, the user wants SPEED.
	// Safe approach: Drop only non-unique performance indexes.

	if err := r.CreateIndexes(); err != nil {
		logger.Error("Failed to create schema/indexes", "error", err)
		panic(err)
	}

	// Recreate triggers to ensure they are up-to-date
	// Note: We only keep the basic insert/delete triggers for FTS.
	// Author, series, and genre linking is handled by RebuildFTSIndex() after bulk imports.
	triggerStmt := `
           DROP TRIGGER IF EXISTS books_fts_insert;
           CREATE TRIGGER books_fts_insert AFTER INSERT ON books BEGIN
               INSERT INTO books_fts(title, author, series, genre, book_id)
               VALUES (
                 new.title,
                 NULL,  -- Author will be populated by RebuildFTSIndex
                 NULL,  -- Series will be populated by RebuildFTSIndex
                 NULL,  -- Genre will be populated by RebuildFTSIndex
                 new.book_id
               );
            END;

           DROP TRIGGER IF EXISTS books_fts_delete;
           CREATE TRIGGER books_fts_delete AFTER DELETE ON books BEGIN
               DELETE FROM books_fts WHERE book_id = old.book_id;
           END;

           -- Drop old triggers that caused performance issues during bulk import
           DROP TRIGGER IF EXISTS books_fts_authors_insert;
           DROP TRIGGER IF EXISTS books_fts_authors_delete;
           DROP TRIGGER IF EXISTS books_fts_series_insert;
           DROP TRIGGER IF EXISTS books_fts_series_delete;
           DROP TRIGGER IF EXISTS books_fts_genres_insert;
           DROP TRIGGER IF EXISTS books_fts_genres_delete;
    `
	_, err = db.Exec(triggerStmt)
	if err != nil {
		logger.Error("Failed to update triggers", "error", err)
		panic(err)
	}

	r.syncGenreDisplayNames()

	return r
}

func (r *Repo) CreateIndexes() error {
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
               name text unique not null,
               display_name text
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

           CREATE VIRTUAL TABLE IF NOT EXISTS books_fts USING fts5(title, author, series, genre, book_id);
  	    `
	_, err := r.db.Exec(sqlStmt)
	return err
}

func (r *Repo) DropIndexes() error {
	// Drop non-unique performance indexes
	sqlStmt := `
		DROP INDEX IF EXISTS I_first_name;
		DROP INDEX IF EXISTS I_last_name;
		DROP INDEX IF EXISTS I_middle_name;
		DROP INDEX IF EXISTS I_title;
		DROP INDEX IF EXISTS I_book_id;
		DROP INDEX IF EXISTS I_author_id;
		DROP INDEX IF EXISTS idx_book_series_book_id;
		DROP INDEX IF EXISTS idx_book_series_series_id;
		DROP INDEX IF EXISTS idx_book_keywords_book_id;
		DROP INDEX IF EXISTS idx_book_keywords_keyword_id;
	`
	_, err := r.db.Exec(sqlStmt)
	return err
}

func (r *Repo) Close() error {
	return r.db.Close()
}

// Ping checks if database connection is alive
func (r *Repo) Ping() error {
	return r.db.Ping()
}

// AddBatch adds multiple books in a single transaction
func (r *Repo) AddBatch(records []*book.Book) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	bi, err := r.NewBatchInserter(tx)
	if err != nil {
		return err
	}
	defer bi.Close()

	for _, record := range records {
		if err := bi.Add(record); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Add adds a single book
func (r *Repo) Add(record *book.Book) error {
	return r.AddBatch([]*book.Book{record})
}

type BatchInserter struct {
	tx                    *sql.Tx
	stmts                 []*sql.Stmt
	stmtInsertBook        *sql.Stmt
	stmtSelectAuthor      *sql.Stmt
	stmtInsertAuthor      *sql.Stmt
	stmtInsertBookAuthor  *sql.Stmt
	stmtSelectGenre       *sql.Stmt
	stmtInsertGenre       *sql.Stmt
	stmtInsertBookGenre   *sql.Stmt
	stmtSelectSeries      *sql.Stmt
	stmtInsertSeries      *sql.Stmt
	stmtInsertBookSeries  *sql.Stmt
	stmtSelectKeyword     *sql.Stmt
	stmtInsertKeyword     *sql.Stmt
	stmtInsertBookKeyword *sql.Stmt
}

func (r *Repo) NewBatchInserter(tx *sql.Tx) (*BatchInserter, error) {
	bi := &BatchInserter{tx: tx}
	var err error

	prepare := func(query string) (*sql.Stmt, error) {
		s, e := tx.Prepare(query)
		if e != nil {
			return nil, e
		}
		bi.stmts = append(bi.stmts, s)
		return s, nil
	}

	if bi.stmtInsertBook, err = prepare(`INSERT INTO books(title, lang, archive, filename, file_size, date_added, lib_id, deleted, lib_rate) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectAuthor, err = prepare(`SELECT author_id FROM authors WHERE first_name = ? AND middle_name = ? AND last_name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertAuthor, err = prepare(`INSERT INTO authors(first_name, middle_name, last_name) VALUES(?, ?, ?) RETURNING author_id`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookAuthor, err = prepare(`INSERT OR IGNORE INTO book_authors(book_id, author_id) VALUES(?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectGenre, err = prepare(`SELECT genre_id FROM genres WHERE name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertGenre, err = prepare(`INSERT INTO genres(name) VALUES(?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookGenre, err = prepare(`INSERT OR IGNORE INTO book_genres(book_id, genre_id) VALUES(?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectSeries, err = prepare(`SELECT series_id FROM series WHERE name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertSeries, err = prepare(`INSERT INTO series(name) VALUES(?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookSeries, err = prepare(`INSERT OR REPLACE INTO book_series (book_id, series_id, series_no) VALUES (?, ?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectKeyword, err = prepare(`SELECT keyword_id FROM keywords WHERE name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertKeyword, err = prepare(`INSERT INTO keywords(name) VALUES(?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookKeyword, err = prepare(`INSERT OR IGNORE INTO book_keywords (book_id, keyword_id) VALUES (?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}

	return bi, nil
}

func (bi *BatchInserter) Close() {
	for _, s := range bi.stmts {
		s.Close()
	}
}

func (bi *BatchInserter) Add(record *book.Book) error {
	authorIDs, err := bi.getOrCreateAuthors(record.Author)
	if err != nil {
		return fmt.Errorf("get or create author: %w", err)
	}

	deletedInt := 0
	if record.Deleted {
		deletedInt = 1
	}

	res, err := bi.stmtInsertBook.Exec(
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
	bookID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get book id: %w", err)
	}

	for _, authorID := range authorIDs {
		if _, err := bi.stmtInsertBookAuthor.Exec(bookID, authorID); err != nil {
			return fmt.Errorf("link author: %w", err)
		}
	}

	if record.Series != nil && record.Series.Name != "" {
		seriesID, err := bi.getOrCreateSeries(record.Series.Name)
		if err != nil {
			return err
		}
		if _, err := bi.stmtInsertBookSeries.Exec(bookID, seriesID, record.Series.SeriesNo); err != nil {
			return fmt.Errorf("link series: %w", err)
		}
	}

	for _, genre := range record.Genres {
		if genre == "" {
			continue
		}
		genreID, err := bi.getOrCreateGenre(genre)
		if err != nil {
			return err
		}
		if _, err := bi.stmtInsertBookGenre.Exec(bookID, genreID); err != nil {
			return fmt.Errorf("link genre: %w", err)
		}
	}

	for _, kw := range record.Keywords {
		if kw == "" {
			continue
		}
		kwID, err := bi.getOrCreateKeyword(kw)
		if err != nil {
			return err
		}
		if _, err := bi.stmtInsertBookKeyword.Exec(bookID, kwID); err != nil {
			return fmt.Errorf("link keyword: %w", err)
		}
	}

	return nil
}

func (bi *BatchInserter) getOrCreateAuthors(authors []book.Author) ([]int64, error) {
	ids := make([]int64, 0, len(authors))
	for _, a := range authors {
		var id int64
		err := bi.stmtSelectAuthor.QueryRow(a.FirstName, a.MiddleName, a.LastName).Scan(&id)
		if err == sql.ErrNoRows {
			err = bi.stmtInsertAuthor.QueryRow(a.FirstName, a.MiddleName, a.LastName).Scan(&id)
		}
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (bi *BatchInserter) getOrCreateSeries(name string) (int64, error) {
	var id int64
	err := bi.stmtSelectSeries.QueryRow(name).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := bi.stmtInsertSeries.Exec(name)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	return id, err
}

func (bi *BatchInserter) getOrCreateGenre(name string) (int64, error) {
	var id int64
	err := bi.stmtSelectGenre.QueryRow(name).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := bi.stmtInsertGenre.Exec(name)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	return id, err
}

func (bi *BatchInserter) getOrCreateKeyword(name string) (int64, error) {
	var id int64
	err := bi.stmtSelectKeyword.QueryRow(name).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := bi.stmtInsertKeyword.Exec(name)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	return id, err
}

func (r *Repo) Search() error {
	return nil
}

func (r *Repo) List() error {
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

func (r *Repo) getOrCreateAuthor(tx *sql.Tx, authors []book.Author) ([]int64, error) {
	if len(authors) < 1 {
		return nil, nil
	}

	result := []int64{}

	selectStmt, err := tx.Prepare(`
		SELECT author_id FROM authors
		WHERE first_name = ? AND middle_name = ? AND last_name = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare select author: %w", err)
	}
	defer selectStmt.Close()

	insertStm, err := tx.Prepare(`
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
		SELECT DISTINCT g.display_name
		FROM genres g
		JOIN book_genres bg ON g.genre_id = bg.genre_id
		JOIN books b ON bg.book_id = b.book_id
		WHERE b.deleted = 0 AND g.display_name IS NOT NULL
		ORDER BY g.display_name
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
			group_concat(distinct a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '')) as author,
			group_concat(distinct g.display_name) as genres
		FROM books_fts fts
		JOIN books b ON fts.book_id = b.book_id
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		LEFT JOIN book_genres bg ON b.book_id = bg.book_id
		LEFT JOIN genres g ON bg.genre_id = g.genre_id
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
		var genresStr sql.NullString

		err := rows.Scan(
			&r.BookID, &r.Title, &r.Lang, &r.Archive, &r.FileName,
			&r.FileSize, &r.Deleted, &seriesName, &seriesNo,
			&r.Rank, &r.Author, &genresStr,
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
		if genresStr.Valid {
			r.Genres = strings.Split(genresStr.String, ",")
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
			(SELECT group_concat(coalesce(g.display_name, g.name), ' | ') 
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

func (r *Repo) syncGenreDisplayNames() {
	rows, err := r.db.Query(`SELECT name, display_name FROM genres`)
	if err != nil {
		logger.Error("Failed to query genres for update", "error", err)
		return
	}

	type genreUpdate struct {
		name        string
		displayName string
	}
	var updates []genreUpdate

	for rows.Next() {
		var name string
		var currentDN sql.NullString
		if err := rows.Scan(&name, &currentDN); err == nil {
			displayName := MapGenre(name)
			if displayName != name && (!currentDN.Valid || currentDN.String != displayName) {
				updates = append(updates, genreUpdate{name, displayName})
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

	stmt, err := tx.Prepare(`UPDATE genres SET display_name = ? WHERE name = ?`)
	if err != nil {
		logger.Error("Failed to prepare statement for genre update", "error", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	count := 0
	for _, u := range updates {
		if _, err := stmt.Exec(u.displayName, u.name); err != nil {
			logger.Error("Failed to execute genre update", "error", err, "name", u.name)
		} else {
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Error("Failed to commit genre updates", "error", err)
	} else if count > 0 {
		logger.Info("Updated genre display names", "new_count", count)
	}
}
