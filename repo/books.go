package repo

import (
	"database/sql"
	"log"

	"github.com/htol/bopds/book"
	_ "github.com/mattn/go-sqlite3"
)

type Repo struct {
	db          *sql.DB
	path        string
	autorsCache map[book.Author]int64
	//authorsTotal uint
	//authorsSeq   uint
}

func GetStorage(path string) *Repo {
	// TODO: parametrize path for sqllitedb
	r := &Repo{
		path:        path,
		autorsCache: make(map[book.Author]int64),
		//authorsTotal: 0,
		//authorsSeq:   0,
	}

	db, err := sql.Open("sqlite3", r.path)
	if err != nil {
		log.Fatal(err)
	}
	r.db = db

	// TODO: Drop indexes
	sqlStmt := `
           CREATE TABLE IF NOT EXISTS "authors" (
               id integer primary key autoincrement not null,
               first_name text,
               middle_name text,
               last_name text,
               UNIQUE(first_name, middle_name, last_name)
           );
           CREATE INDEX IF NOT EXISTS [I_first_name] ON "authors" ([first_name]);
           CREATE INDEX IF NOT EXISTS [I_last_name] ON "authors" ([last_name]);
           CREATE INDEX IF NOT EXISTS [I_middle_name] ON "authors" ([middle_name]);

           CREATE TABLE IF NOT EXISTS "books" (
                id integer primary key autoincrement not null,
                title text,
                authors integer,
                lang text,
                archive text,
                filename text
            );
           CREATE INDEX IF NOT EXISTS [I_title] ON "books" ([title]);
           CREATE INDEX IF NOT EXISTS [I_authors] ON "books" ([authors]);

           CREATE TABLE IF NOT EXISTS "book_authors" (
               id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
               book_id INTEGER NOT NULL,
               author_id INTEGER NOT NULL,
               FOREIGN KEY (book_id) REFERENCES books(id),
               FOREIGN KEY (author_id) REFERENCES authors(id)
           );
           CREATE INDEX IF NOT EXISTS [I_book_id] ON "book_authors" ([book_id]);
           CREATE INDEX IF NOT EXISTS [I_author_id] ON "book_authors" ([author_id]);
	    `
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, sqlStmt)
	}

	r.initAuthorsCache()

	return r
}

func (r *Repo) Close() {
	r.db.Close()
}

func (r *Repo) initAuthorsCache() error {
	QUERY := `SELECT id, first_name, middle_name, last_name FROM authors`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var a book.Author
		var authorId int64

		rows.Scan(&authorId, &a.FirstName, &a.MiddleName, &a.LastName)
		r.autorsCache[a] = authorId
	}

	return nil
}

func (r *Repo) Add(record *book.Book) error {
	authorsIDs := r.getOrCreateAuthor(record.Author) // TODO: plural name?
	//log.Printf("%#v", authorsIDs)
	//return nil

	INSERT_BOOK := `INSERT INTO books(title, lang, archive, filename) VALUES(?, ?, ?, ?)`
	insertStm, err := r.db.Prepare(INSERT_BOOK)
	if err != nil {
		log.Fatal(err)
	}
	sqlresult, err := insertStm.Exec(record.Title, record.Lang, record.Archive, record.FileName)
	if err != nil {
		log.Fatal(err)
	}
	bookID, _ := sqlresult.LastInsertId()

	tx, err := r.db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO book_authors(book_id, author_id) VALUES(?, ?)`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	for _, author := range authorsIDs {
		_, err := stmt.Exec(bookID, author)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
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

func (r *Repo) getOrCreateAuthor(authors []book.Author) []int64 {
	if len(authors) < 1 {
		return nil
	}

	result := []int64{}
	// TODO: BUG: will not return id if already in table
	// TODO: probably have to rework this part
	// 1. autorsCache should be initially populated from db
	// 2. if no authoer in cache then create without ignore
	INSERT_AUTHOR := `INSERT INTO authors(first_name, middle_name, last_name) VALUES(?, ?, ?) RETURNING id`
	insertStm, err := r.db.Prepare(INSERT_AUTHOR)
	if err != nil {
		log.Fatal(err)
	}
	defer insertStm.Close()

	for _, author := range authors {
		id, ok := r.autorsCache[author]
		if ok {
			result = append(result, id)
			continue
		}
		sqlresult, err := insertStm.Exec(author.FirstName, author.MiddleName, author.LastName)
		if err != nil {
			log.Fatal(err)
		}
		id, _ = sqlresult.LastInsertId()
		//log.Printf("#%v #%v", id, err)
		r.autorsCache[author] = id
		//log.Printf("%d:%d %#v\n", r.authorsTotal, id, author)
	}
	return result
}

func (r *Repo) GetAuthors() ([]book.Author, error) {
	QUERY := `SELECT first_name, middle_name, last_name FROM authors`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author

		rows.Scan(&a.FirstName, &a.MiddleName, &a.LastName)
		authors = append(authors, a)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return authors, nil
}
