package repo

import (
	"encoding/xml"
	"fmt"
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

func TestLibraryScanSession_TombstonesMissing(t *testing.T) {
	dbPath := "./test_tombstone.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	libID, err := db.GetOrCreateLibrary("lib", "")
	if err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	makeBook := func(libID int64, title string) *book.Book {
		return &book.Book{
			Library:  "lib",
			LibID:    libID,
			Title:    title,
			Archive:  "a.zip",
			FileName: fmt.Sprintf("%d.fb2", libID),
			Lang:     "en",
		}
	}

	addAll := func(ids ...int64) error {
		books := make([]*book.Book, 0, len(ids))
		titles := map[int64]string{1: "Aaa Book", 2: "Bbb Book", 3: "Ccc Book", 4: "Ddd Book"}
		for _, id := range ids {
			books = append(books, makeBook(id, titles[id]))
		}
		return db.AddBatch(books)
	}

	// First scan: all four books
	s1 := db.BeginLibraryScan(libID)
	if err := addAll(1, 2, 3, 4); err != nil {
		t.Fatalf("AddBatch (scan 1) failed: %v", err)
	}
	if err := s1.Finish(); err != nil {
		t.Fatalf("Finish (scan 1) failed: %v", err)
	}

	booksByID := func() map[int64]book.Book {
		t.Helper()
		books, err := db.GetBooks()
		if err != nil {
			t.Fatalf("GetBooks failed: %v", err)
		}
		result := make(map[int64]book.Book)
		for _, row := range books {
			var id int64
			if _, err := fmt.Sscanf(row, "%d,", &id); err != nil {
				t.Fatalf("Cannot parse book row %q: %v", row, err)
			}
			var b book.Book
			if err := db.db.QueryRow(`SELECT book_id, lib_id FROM books WHERE book_id = ?`, id).Scan(&b.BookID, &b.LibID); err != nil {
				t.Fatalf("Select book failed: %v", err)
			}
			result[b.LibID] = b
		}
		return result
	}

	visible := booksByID()
	if len(visible) != 4 {
		t.Fatalf("Expected 4 visible books after scan 1, got %d", len(visible))
	}
	origDID := visible[4].BookID

	// Second scan: book 4 absent from the new data set
	s2 := db.BeginLibraryScan(libID)
	if err := addAll(1, 2, 3); err != nil {
		t.Fatalf("AddBatch (scan 2) failed: %v", err)
	}
	if err := s2.Finish(); err != nil {
		t.Fatalf("Finish (scan 2) failed: %v", err)
	}

	var deletedFlag bool
	if err := db.db.QueryRow(`SELECT deleted FROM books WHERE book_id = ?`, origDID).Scan(&deletedFlag); err != nil {
		t.Fatalf("Select deleted flag failed: %v", err)
	}
	if !deletedFlag {
		t.Errorf("Missing book must be tombstoned (deleted=1), got %v", deletedFlag)
	}
	visible = booksByID()
	if len(visible) != 3 {
		t.Errorf("Tombstoned book must be hidden, got %d visible", len(visible))
	}

	// Third scan: book 4 returns
	s3 := db.BeginLibraryScan(libID)
	if err := addAll(1, 2, 3, 4); err != nil {
		t.Fatalf("AddBatch (scan 3) failed: %v", err)
	}
	if err := s3.Finish(); err != nil {
		t.Fatalf("Finish (scan 3) failed: %v", err)
	}

	visible = booksByID()
	if len(visible) != 4 {
		t.Fatalf("Expected 4 visible books after scan 3, got %d", len(visible))
	}
	if visible[4].BookID != origDID {
		t.Errorf("Returned book must keep its book_id: got %d, want %d", visible[4].BookID, origDID)
	}
	if err := db.db.QueryRow(`SELECT deleted FROM books WHERE book_id = ?`, origDID).Scan(&deletedFlag); err != nil {
		t.Fatalf("Select deleted flag (scan 3) failed: %v", err)
	}
	if deletedFlag {
		t.Errorf("Returned book must be un-tombstoned, got deleted=%v", deletedFlag)
	}
}

func TestLibraryScanSession_ThresholdGuard(t *testing.T) {
	// 2 books, 1 missing on rescan = 50% > default threshold 30% -> guard trips
	dbPath := "./test_threshold.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	libID, err := db.GetOrCreateLibrary("lib", "")
	if err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	makeBook := func(libID int64, title string) *book.Book {
		return &book.Book{
			Library:  "lib",
			LibID:    libID,
			Title:    title,
			Archive:  "a.zip",
			FileName: fmt.Sprintf("%d.fb2", libID),
			Lang:     "en",
		}
	}

	s1 := db.BeginLibraryScan(libID)
	if err := db.AddBatch([]*book.Book{makeBook(1, "Aaa Book"), makeBook(2, "Bbb Book")}); err != nil {
		t.Fatalf("AddBatch (scan 1) failed: %v", err)
	}
	if err := s1.Finish(); err != nil {
		t.Fatalf("Finish (scan 1) failed: %v", err)
	}

	// Scan 2 sees only book 1: more than 30% of rows would be tombstoned
	s2 := db.BeginLibraryScan(libID)
	if err := db.AddBatch([]*book.Book{makeBook(1, "Aaa Book")}); err != nil {
		t.Fatalf("AddBatch (scan 2) failed: %v", err)
	}
	err = s2.Finish()
	if err == nil {
		t.Fatal("Expected threshold guard error, got none")
	}

	// Nothing was tombstoned: both books still visible
	books, err := db.GetBooksByLetter("B")
	if err != nil {
		t.Fatalf("GetBooksByLetter failed: %v", err)
	}
	if len(books) != 1 {
		t.Errorf("Guard must abort without tombstoning: expected 1 visible 'Bbb Book', got %d", len(books))
	}
}

func TestLibraryScanSession_ThresholdDisabled(t *testing.T) {
	t.Setenv("LIBRARY_MISSING_THRESHOLD", "100")

	dbPath := "./test_threshold_off.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	libID, err := db.GetOrCreateLibrary("lib", "")
	if err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	makeBook := func(libID int64, title string) *book.Book {
		return &book.Book{
			Library:  "lib",
			LibID:    libID,
			Title:    title,
			Archive:  "a.zip",
			FileName: fmt.Sprintf("%d.fb2", libID),
			Lang:     "en",
		}
	}

	s1 := db.BeginLibraryScan(libID)
	if err := db.AddBatch([]*book.Book{makeBook(1, "Aaa Book"), makeBook(2, "Bbb Book")}); err != nil {
		t.Fatalf("AddBatch (scan 1) failed: %v", err)
	}
	if err := s1.Finish(); err != nil {
		t.Fatalf("Finish (scan 1) failed: %v", err)
	}

	// Threshold 100 disables the guard: the same flow tombstones freely
	s2 := db.BeginLibraryScan(libID)
	if err := db.AddBatch([]*book.Book{makeBook(1, "Aaa Book")}); err != nil {
		t.Fatalf("AddBatch (scan 2) failed: %v", err)
	}
	if err := s2.Finish(); err != nil {
		t.Fatalf("Finish with guard disabled must not fail: %v", err)
	}

	books, err := db.GetBooksByLetter("B")
	if err != nil {
		t.Fatalf("GetBooksByLetter failed: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("Expected 'Bbb Book' tombstoned with guard disabled, got %d books", len(books))
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
