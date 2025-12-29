package repo

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/htol/bopds/book"
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
	// TODO: parametrize path for sqllitedb
	r := &Repo{
		path:         path,
		AuthorsCache: make(map[book.Author]int64),
		//authorsTotal: 0,
		//authorsSeq:   0,
	}

	db, err := sql.Open("sqlite3", "file:"+r.path+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(1)
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
		log.Fatalf("%q: %s\n", err, sqlStmt)
	}

	if err := r.initAuthorsCache(); err != nil {
		log.Fatalf("init authors cache: %v", err)
	}

	return r
}

func (r *Repo) Close() error {
	return r.db.Close()
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
	QUERY := `SELECT first_name, middle_name, last_name FROM authors`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query authors: %w", err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author

		if err := rows.Scan(&a.FirstName, &a.MiddleName, &a.LastName); err != nil {
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
	QUERY := `SELECT first_name, middle_name, last_name FROM authors WHERE last_name LIKE ? COLLATE NOCASE ORDER BY last_name`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query authors by letter: %w", err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author

		if err := rows.Scan(&a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return nil, fmt.Errorf("scan author by letter: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors by letter: %w", err)
	}

	return authors, nil
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
	QUERY := `SELECT  Title FROM books WHERE title LIKE ? COLLATE NOCASE ORDER BY title`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query books by letter: %w", err)
	}
	defer rows.Close()

	books := make([]book.Book, 0)
	for rows.Next() {
		var a book.Book

		if err := rows.Scan(&a.Title); err != nil {
			return nil, fmt.Errorf("scan book by letter: %w", err)
		}
		books = append(books, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by letter: %w", err)
	}

	return books, nil
}
func (r *Repo) GetBooksByAuthorID(id int64) ([]string, error) {
	QUERY := `
SELECT *
FROM books
LEFT JOIN book_authors
ON books.book_id = book_authors.book_id
LEFT JOIN authors
ON book_authors.author_id = authors.author_id
WHERE author_id=?
`

	rows, err := r.db.Query(QUERY, id)
	if err != nil {
		return nil, fmt.Errorf("query books by author id: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get book columns by author id: %w", err)
	}

	books := make([]string, 0)

	for rows.Next() {
		columns := make([]sql.NullString, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, fmt.Errorf("scan book by author id: %w", err)
		}
		var sb strings.Builder
		for i := range cols {
			fmt.Fprintf(&sb, "%s, ", columns[i].String)
		}

		books = append(books, sb.String())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by author id: %w", err)
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
