package repo

import (
	"bytes"
	"encoding/xml"
	"log"
	"os"
	"testing"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
)

func init() {
	logger.Init("info")
}

// cleanupTestDB removes the test database and any SQLite WAL files
func cleanupTestDB(path string) {
	os.Remove(path)
	os.Remove(path + "-shm")
	os.Remove(path + "-wal")
}

// getOrCreateAuthorHelper wraps the internal getOrCreateAuthor in a transaction for testing
func getOrCreateAuthorHelper(db *Repo, authors []book.Author) ([]int64, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	ids, err := db.getOrCreateAuthor(tx, authors)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}

func TestGetOrCreateAuthor(t *testing.T) {
	dbPath := "./test.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()
	authors := []book.Author{
		{
			XMLName:    xml.Name{},
			FirstName:  "Василий",
			MiddleName: "Петрович",
			LastName:   "Иванов"},
	}
	authorIDs, err := getOrCreateAuthorHelper(db, authors)
	if err != nil {
		t.Fatalf("getOrCreateAuthor failed: %v", err)
	}
	if len(authorIDs) != 1 {
		t.Fatalf("expected 1 author ID, got %d", len(authorIDs))
	}
}

func TestAdd(t *testing.T) {
	dbPath := "./test.db"
	cleanupTestDB(dbPath)

	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	book := &book.Book{XMLName: xml.Name{Space: "", Local: ""}, Author: []book.Author{{XMLName: xml.Name{Space: "", Local: ""}, FirstName: "Пьер", MiddleName: "", LastName: "Абеляр"}}, Title: "История моих бедствий", Lang: "ru", Genres: []string{"sci_philosophy"}, Archive: "", FileName: "125.fb2"}

	db.Add(book)

	t.Log(buf.String())
}
