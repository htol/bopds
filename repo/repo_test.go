package repo

import (
	"encoding/xml"
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
func getOrCreateAuthorHelper(t testing.TB, db *Repo, authors []book.Author) ([]int64, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			t.Logf("Failed to rollback transaction: %v", err)
		}
	}()
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
	authorIDs, err := getOrCreateAuthorHelper(t, db, authors)
	if err != nil {
		t.Fatalf("getOrCreateAuthor failed: %v", err)
	}
	if len(authorIDs) != 1 {
		t.Fatalf("expected 1 author ID, got %d", len(authorIDs))
	}
}

func TestGetOrCreateLibrary(t *testing.T) {
	dbPath := "./test_libraries.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)

	id1, err := db.GetOrCreateLibrary("libA", "Library Alpha")
	if err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	id2, err := db.GetOrCreateLibrary("libA", "Library Alpha")
	if err != nil {
		t.Fatalf("GetOrCreateLibrary (second call) failed: %v", err)
	}
	if id1 != id2 {
		t.Errorf("Same name must return same ID: got %d and %d", id1, id2)
	}

	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM libraries WHERE name = 'libA'`).Scan(&count); err != nil {
		t.Fatalf("Count libraries failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 libraries row, got %d", count)
	}

	var displayName string
	if err := db.db.QueryRow(`SELECT display_name FROM libraries WHERE library_id = ?`, id1).Scan(&displayName); err != nil {
		t.Fatalf("Select display_name failed: %v", err)
	}
	if displayName != "Library Alpha" {
		t.Errorf("Expected display_name 'Library Alpha', got %q", displayName)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// A fresh Repo must not erase the stored display name when given an empty one
	db2 := GetStorage(dbPath)
	defer func() {
		db2.Close()
		cleanupTestDB(dbPath)
	}()
	if _, err := db2.GetOrCreateLibrary("libA", ""); err != nil {
		t.Fatalf("GetOrCreateLibrary (reopen) failed: %v", err)
	}
	if err := db2.db.QueryRow(`SELECT display_name FROM libraries WHERE name = 'libA'`).Scan(&displayName); err != nil {
		t.Fatalf("Select display_name after reopen failed: %v", err)
	}
	if displayName != "Library Alpha" {
		t.Errorf("Empty display name must keep stored value, got %q", displayName)
	}
}

func TestAddBatch_UpsertByLibId(t *testing.T) {
	dbPath := "./test_upsert.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	if _, err := db.GetOrCreateLibrary("lib", ""); err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	makeBook := func(title string) *book.Book {
		return &book.Book{
			Library:  "lib",
			Title:    title,
			LibID:    42,
			Archive:  "a.zip",
			FileName: "42.fb2",
			Lang:     "en",
		}
	}

	if err := db.AddBatch([]*book.Book{makeBook("First Title")}); err != nil {
		t.Fatalf("AddBatch (insert) failed: %v", err)
	}

	books, err := db.GetBooksByLetter("F")
	if err != nil {
		t.Fatalf("GetBooksByLetter failed: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("Expected 1 book after insert, got %d", len(books))
	}
	origID := books[0].BookID

	// Same (library, lib_id), different title: one row, same book_id, title updated
	if err := db.AddBatch([]*book.Book{makeBook("Second Title")}); err != nil {
		t.Fatalf("AddBatch (upsert) failed: %v", err)
	}

	books, err = db.GetBooksByLetter("S")
	if err != nil {
		t.Fatalf("GetBooksByLetter failed: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("Expected 1 book after upsert, got %d", len(books))
	}
	if books[0].BookID != origID {
		t.Errorf("book_id must survive the upsert: got %d, want %d", books[0].BookID, origID)
	}

	books, err = db.GetBooksByLetter("F")
	if err != nil {
		t.Fatalf("GetBooksByLetter (old title) failed: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("Old title must be gone after update, got %d books", len(books))
	}

	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM books`).Scan(&count); err != nil {
		t.Fatalf("Count books failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 books row, got %d", count)
	}
}

func TestAddBatch_UpsertByFilename(t *testing.T) {
	dbPath := "./test_upsert_file.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	if _, err := db.GetOrCreateLibrary("lib", ""); err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	// lib_id empty (0): fallback natural key is (library_id, archive, filename)
	makeBook := func(title string) *book.Book {
		return &book.Book{
			Library:  "lib",
			Title:    title,
			Archive:  "a.zip",
			FileName: "no-libid.fb2",
			Lang:     "en",
		}
	}

	if err := db.AddBatch([]*book.Book{makeBook("Alpha Book")}); err != nil {
		t.Fatalf("AddBatch (insert) failed: %v", err)
	}
	if err := db.AddBatch([]*book.Book{makeBook("Beta Book")}); err != nil {
		t.Fatalf("AddBatch (upsert) failed: %v", err)
	}

	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM books`).Scan(&count); err != nil {
		t.Fatalf("Count books failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 books row after fallback-key upsert, got %d", count)
	}

	books, err := db.GetBooksByLetter("B")
	if err != nil {
		t.Fatalf("GetBooksByLetter failed: %v", err)
	}
	if len(books) != 1 || books[0].Title != "Beta Book" {
		t.Errorf("Expected updated 'Beta Book', got %+v", books)
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

	book := &book.Book{XMLName: xml.Name{Space: "", Local: ""}, Author: []book.Author{{XMLName: xml.Name{Space: "", Local: ""}, FirstName: "Пьер", MiddleName: "", LastName: "Абеляр"}}, Title: "История моих бедствий", Lang: "ru", Genres: []string{"sci_philosophy"}, Archive: "", FileName: "125.fb2"}

	if err := db.Add(book); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

}
