package repo

import (
	"database/sql"
	"log"
	"os"

	"github.com/htol/bopds/book"
	_ "github.com/mattn/go-sqlite3"
)

type Repo struct {
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

	if _, err := os.Stat(r.path); os.IsNotExist(err) {
		db, err := sql.Open("sqlite3", r.path)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		// TODO: Drop indexes
		sqlStmt := `
           DROP TABLE IF EXISTS authors;
           DROP TABLE IF EXISTS books;
           DROP TABLE IF EXISTS book_authors;

           CREATE TABLE IF NOT EXISTS "authors" (
               id integer primary key autoincrement not null,
               first_name text,
               middle_name text,
               last_name text,
               UNIQUE(first_name, middle_name, last_name)
           );
           CREATE INDEX [I_first_name] ON "authors" ([first_name]);
           CREATE INDEX [I_last_name] ON "authors" ([last_name]);
           CREATE INDEX [I_middle_name] ON "authors" ([middle_name]);

           CREATE TABLE IF NOT EXISTS "books" (
                id integer primary key autoincrement not null,
                title text,
                authors integer,
                lang text,
                archive text,
                filename text
            );
           CREATE INDEX [I_title] ON "books" ([title]);
           CREATE INDEX [I_authors] ON "books" ([authors]);

           CREATE TABLE IF NOT EXISTS "book_authors" (
               id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
               book_id INTEGER NOT NULL,
               author_id INTEGER NOT NULL,
               FOREIGN KEY (book_id) REFERENCES books(id),
               FOREIGN KEY (author_id) REFERENCES authors(id)
           );
           CREATE INDEX [I_book_id] ON "book_authors" ([book_id]);
           CREATE INDEX [I_author_id] ON "book_authors" ([author_id]);
	    `
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Fatalf("%q: %s\n", err, sqlStmt)
		}
	}

	r.initAuthorsCache()

	return r
}

func (r *Repo) initAuthorsCache() error {
	db, err := sql.Open("sqlite3", r.path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	QUERY := `SELECT id, first_name, middle_name, last_name FROM authors`

	rows, err := db.Query(QUERY)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var a book.Author
		var bookID int64

		rows.Scan(&bookID, &a.FirstName, &a.MiddleName, &a.LastName)
		r.autorsCache[a] = bookID
	}

	return nil
}

func (r *Repo) Add(record *book.Book) error {
	db, err := sql.Open("sqlite3", r.path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	authorsIDs := r.getOrCreateAuthor(record.Author) // TODO: plural name?
	//log.Printf("%#v", authorsIDs)
	//return nil

	INSERT_BOOK := `INSERT INTO books(title, lang, archive, filename) VALUES(?, ?, ?, ?)`
	insertStm, err := db.Prepare(INSERT_BOOK)
	if err != nil {
		log.Fatal(err)
	}
	sqlresult, err := insertStm.Exec(record.Title, record.Lang, record.Archive, record.FileName)
	if err != nil {
		log.Fatal(err)
	}
	bookID, _ := sqlresult.LastInsertId()

	tx, err := db.Begin()
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
//	db, err := sql.Open("sqlite3", b.path)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer db.Close()
//
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
	db, err := sql.Open("sqlite3", r.path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	result := []int64{}
	// TODO: BUG: will not return id if already in table
	// TODO: probably have to rework this part
	// 1. autorsCache should be initially populated from db
	// 2. if no authoer in cache then create without ignore
	INSERT_AUTHOR := `INSERT INTO authors(first_name, middle_name, last_name) VALUES(?, ?, ?) RETURNING id`
	insertStm, err := db.Prepare(INSERT_AUTHOR)
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
