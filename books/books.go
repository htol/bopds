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
	        create table foo (id integer not null primary key, name text);
	        delete from foo;
	    `
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Fatalf("%q: %s\n", err, sqlStmt)
		}
	}

	return b
}

func (b *Books) List() {

}
