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
	        CREATE TABLE IF NOT EXISTS "books" (
                id integer primary key autoincrement not null,
                first_name text,
                last_name text,
                title text not null,
                path test not null,
                fname text not null,
                archive bool
            );
           CREATE INDEX [I_first_name] ON "books" ([first_name]);
           CREATE INDEX [I_last_name] ON "books" ([last_name]);
           CREATE INDEX [I_title] ON "books" ([title]);
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
