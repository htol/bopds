package repo

import (
	"bytes"
	"encoding/xml"
	"log"
	"os"
	"testing"

	"github.com/htol/bopds/book"
)

func TestGetOrCreateAuthor(t *testing.T) {
	os.Remove("./test.db")
	db := GetStorage("test.db")
	defer db.Close()
	authors := []book.Author{
		{
			XMLName:    xml.Name{},
			FirstName:  "Василий",
			MiddleName: "Петрович",
			LastName:   "Иванов"},
	}
	authorIDs, err := db.getOrCreateAuthor(authors)
	if err != nil {
		t.Fatalf("getOrCreateAuthor failed: %v", err)
	}
	if len(authorIDs) != 1 {
		t.Fatalf("expected 1 author ID, got %d", len(authorIDs))
	}
}

func TestAdd(t *testing.T) {
	os.Remove("./test.db")

	db := GetStorage("test.db")
	defer db.Close()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	book := &book.Book{XMLName: xml.Name{Space: "", Local: ""}, Author: []book.Author{{XMLName: xml.Name{Space: "", Local: ""}, FirstName: "Пьер", MiddleName: "", LastName: "Абеляр"}}, Title: "История моих бедствий", Lang: "ru", Genres: []string{"sci_philosophy"}, Archive: "", FileName: "125.fb2"}

	db.Add(book)

	t.Log(buf.String())
}
