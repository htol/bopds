package repo

import (
	"encoding/xml"
	"testing"

	"github.com/htol/bopds/book"
)

func TestGetOrCreateAuthor(t *testing.T) {
	db := GetStorage("test.db")
	authors := []book.Author{
		book.Author{
			XMLName:    xml.Name{},
			FirstName:  "Василий",
			MiddleName: "Петрович",
			LastName:   "Иванов"},
	}
	db.getOrCreateAuthor(authors)
	t.Logf("#%v", db.AuthorsCache)

}
