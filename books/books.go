package books

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
)

type Books struct {
	path string
}

func GetStorage() *Books {
	b := &Books{
		path: "books.db",
	}

	if _, err := os.Stat(b.path); os.IsNotExist(err) {
		db, err := sql.Open("sqlite3", b.path)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		sqlStmt := `
            DROP TABLE IF EXISTS authors;
            DROP TABLE IF EXISTS books;
            DROP TABLE IF EXISTS book_authors;
            // TODO: Drop indexes

	        CREATE TABLE IF NOT EXISTS "authors" (
                id integer primary key autoincrement not null,
                first_name text,
                last_name text,
            );
           CREATE INDEX [I_first_name] ON "authors" ([first_name]);
           CREATE INDEX [I_last_name] ON "authors" ([last_name]);
           CREATE INDEX [I_middle_name ON "authors" ([middle_name]);

           CREATE TABLE IF NOT EXISTS "books" (
                id integer primary key autoincrement not null,
                title text,
                authors integer,
            );
           CREATE INDEX [I_title] ON "books" ([title]);
           CREATE INDEX [I_authors] ON "books" ([authors]);

           CREATE TABLE IF NOT EXISTS "book_authors" (
               id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
               book_id INTEGER NOT NULL,
               autor_id INTEGER NOT NULL,
               FOREIGN KEY (book_id) REFERENCES books(id),
               FOREIGN KEY (autor_id) REFERENCES authors(id),
           );
           CREATE INDEX [I_book_id] ON "book_authors" ([book_id]);
           CREATE INDEX [I_author_id] ON "book_authors" ([autor_id]);
	    `
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Fatalf("%q: %s\n", err, sqlStmt)
		}
	}

	return b
}

func (b *Books) Add() {

}

func (b *Books) List() {

}
