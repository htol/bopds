package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/config"
	"github.com/htol/bopds/logger"
	_ "github.com/mattn/go-sqlite3"
)

type Repo struct {
	db           *sql.DB
	path         string
	AuthorsCache map[book.Author]int64
	//authorsTotal uint
	//authorsSeq   uint
}

// Ensure Repo implements Repository interface
var _ Repository = (*Repo)(nil)

func GetStorage(path string) *Repo {
	return GetStorageWithConfig(path, config.Load())
}

func GetStorageWithConfig(path string, cfg *config.Config) *Repo {
	r := &Repo{
		path:         path,
		AuthorsCache: make(map[book.Author]int64),
		//authorsTotal: 0,
		//authorsSeq:   0,
	}

	db, err := sql.Open("sqlite3", "file:"+r.path+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		logger.Error("Failed to open database", "path", r.path, "error", err)
		panic(err)
	}

	// Configure connection pool from config
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
                filename text
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
	    `
	_, err = db.Exec(sqlStmt)
	if err != nil {
		logger.Error("Failed to execute schema", "error", err)
		panic(err)
	}

	if err := r.initAuthorsCache(); err != nil {
		logger.Error("Failed to initialize authors cache", "error", err)
		panic(err)
	}

	return r
}

func (r *Repo) Close() error {
	return r.db.Close()
}

// Ping checks if the database connection is alive
func (r *Repo) Ping() error {
	return r.db.Ping()
}

func (r *Repo) initAuthorsCache() error {
	QUERY := `SELECT author_id, first_name, middle_name, last_name FROM authors`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return fmt.Errorf("query authors cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a book.Author
		var authorId int64

		if err := rows.Scan(&authorId, &a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return fmt.Errorf("scan author cache: %w", err)
		}
		r.AuthorsCache[a] = authorId
	}

	return nil
}

func (r *Repo) Add(record *book.Book) error {
	authorsIDs, err := r.getOrCreateAuthor(record.Author)
	if err != nil {
		return fmt.Errorf("get or create author: %w", err)
	}

	INSERT_BOOK := `INSERT INTO books(title, lang, archive, filename) VALUES(?, ?, ?, ?)`
	insertStm, err := r.db.Prepare(INSERT_BOOK)
	if err != nil {
		return fmt.Errorf("prepare insert book: %w", err)
	}
	defer insertStm.Close()

	sqlresult, err := insertStm.Exec(record.Title, record.Lang, record.Archive, record.FileName)
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

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO book_authors(book_id, author_id) VALUES(?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare book_authors: %w", err)
	}
	defer stmt.Close()

	for _, author := range authorsIDs {
		if _, err := stmt.Exec(bookID, author); err != nil {
			tx.Rollback()
			return fmt.Errorf("insert book_author: %w", err)
		}
	}

	if len(record.Genres) > 0 {
		getGenreStmt, err := tx.Prepare(`SELECT genre_id FROM genres WHERE name = ?`)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("prepare get genre: %w", err)
		}
		defer getGenreStmt.Close()

		insertGenreStmt, err := tx.Prepare(`INSERT INTO genres(name) VALUES(?)`)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("prepare insert genre: %w", err)
		}
		defer insertGenreStmt.Close()

		bookGenreStmt, err := tx.Prepare(`INSERT OR IGNORE INTO book_genres(book_id, genre_id) VALUES(?, ?)`)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("prepare book_genre: %w", err)
		}
		defer bookGenreStmt.Close()

		for _, genre := range record.Genres {
			if genre == "" {
				continue
			}

			var genreID int64
			err := getGenreStmt.QueryRow(genre).Scan(&genreID)
			if err == sql.ErrNoRows {
				result, err := insertGenreStmt.Exec(genre)
				if err != nil {
					tx.Rollback()
					return fmt.Errorf("insert genre: %w", err)
				}
				genreID, err = result.LastInsertId()
				if err != nil {
					tx.Rollback()
					return fmt.Errorf("get genre id: %w", err)
				}
			} else if err != nil {
				tx.Rollback()
				return fmt.Errorf("get genre id: %w", err)
			}

			if _, err := bookGenreStmt.Exec(bookID, genreID); err != nil {
				tx.Rollback()
				return fmt.Errorf("insert book_genre: %w", err)
			}
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

//func (b *Books) BulkInsert(records []*book.Book) {
//	tx, err := db.Begin()
//	if err != nil {
//		log.Fatal(err)
//	}
//  /
//	INSERT_BOOK := `INSERT INTO books(title, lang, archive, filename) VALUES(?, ?, ?, ?)`
//	insertStm, err := db.Prepare(INSERT_BOOK)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for _, record := range records {
//		_, err = insertStm.Exec(record.Title, record.Lang, record.Archive, record.FileName)
//		if err != nil {
//			log.Fatal(err)
//		}
//	}
//
//	tx.Commit()
//}

func (r *Repo) getOrCreateAuthor(authors []book.Author) ([]int64, error) {
	if len(authors) < 1 {
		return nil, nil
	}

	result := []int64{}
	INSERT_AUTHOR := `INSERT INTO authors(first_name, middle_name, last_name) VALUES(?, ?, ?) RETURNING author_id`
	insertStm, err := r.db.Prepare(INSERT_AUTHOR)
	if err != nil {
		return nil, fmt.Errorf("prepare insert author: %w", err)
	}
	defer insertStm.Close()

	for _, author := range authors {
		id, ok := r.AuthorsCache[author]
		if ok {
			result = append(result, id)
			continue
		}
		sqlresult, err := insertStm.Exec(author.FirstName, author.MiddleName, author.LastName)
		if err != nil {
			return nil, fmt.Errorf("insert author: %w", err)
		}
		id, err = sqlresult.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("get author id: %w", err)
		}
		r.AuthorsCache[author] = id
		result = append(result, id)
	}
	return result, nil
}

func (r *Repo) GetAuthors() ([]book.Author, error) {
	QUERY := `SELECT author_id, first_name, middle_name, last_name FROM authors`

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
	QUERY := `SELECT author_id, first_name, middle_name, last_name FROM authors WHERE last_name LIKE ? COLLATE NOCASE ORDER BY last_name`

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
			   COUNT(ba.book_id) as book_count
		FROM authors a
		LEFT JOIN book_authors ba ON a.author_id = ba.author_id
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
			   COUNT(ba.book_id) as book_count
		FROM authors a
		LEFT JOIN book_authors ba ON a.author_id = ba.author_id
		WHERE a.last_name LIKE ? COLLATE NOCASE
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
			return nil, fmt.Errorf("scan author with count: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors with count: %w", err)
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
	QUERY := `SELECT * FROM books`

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
	QUERY := `SELECT book_id, Title FROM books WHERE title LIKE ? COLLATE NOCASE ORDER BY title`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query books by letter: %w", err)
	}
	defer rows.Close()

	books := make([]book.Book, 0)
	for rows.Next() {
		var a book.Book

		if err := rows.Scan(&a.BookID, &a.Title); err != nil {
			return nil, fmt.Errorf("scan book by letter: %w", err)
		}
		books = append(books, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by letter: %w", err)
	}

	return books, nil
}
func (r *Repo) GetBooksByAuthorID(id int64) ([]book.Book, error) {
	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   a.first_name, a.middle_name, a.last_name
		FROM books b
		JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		WHERE ba.author_id = ?
		ORDER BY b.title
	`

	rows, err := r.db.Query(QUERY, id)
	if err != nil {
		return nil, fmt.Errorf("query books by author id: %w", err)
	}
	defer rows.Close()

	// Use map to collect books with multiple authors
	booksMap := make(map[int64]*book.Book)
	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString

		if err := rows.Scan(&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&firstName, &middleName, &lastName); err != nil {
			return nil, fmt.Errorf("scan book by author id: %w", err)
		}

		// Check if book already exists in map
		if existingBook, ok := booksMap[b.BookID]; ok {
			// Add author to existing book
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}
		} else {
			// Create new book entry
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			booksMap[b.BookID] = &b
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by author id: %w", err)
	}

	// Convert map to slice
	books := make([]book.Book, 0, len(booksMap))
	for _, book := range booksMap {
		books = append(books, *book)
	}

	return books, nil
}

func (r *Repo) GetGenres() ([]string, error) {
	QUERY := `SELECT name FROM genres ORDER BY name`

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
	QUERY := `SELECT book_id, title, lang, archive, filename FROM books WHERE book_id = ?`

	var b book.Book
	err := r.db.QueryRow(QUERY, id).Scan(&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get book by ID %d: %w", id, err)
	}

	// Fetch authors for this book
	authorsQuery := `
		SELECT a.first_name, a.middle_name, a.last_name
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		WHERE ba.book_id = ?
		ORDER BY a.last_name, a.first_name
	`

	rows, err := r.db.Query(authorsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("query authors for book %d: %w", id, err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author
		if err := rows.Scan(&a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return nil, fmt.Errorf("scan author for book %d: %w", id, err)
		}
		authors = append(authors, a)
	}

	b.Author = authors
	return &b, nil
}
