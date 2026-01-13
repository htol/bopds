package repo

import (
	"encoding/xml"
	"os"
	"testing"
	"time"

	"github.com/htol/bopds/book"
)

func TestGetAuthorsByLetter_FiltersDeletedBooks(t *testing.T) {
	dbPath := "./test_author_visibility.db"
	os.Remove(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		os.Remove(dbPath)
	}()

	// 1. Create an author with only deleted books
	deletedAuthor := book.Author{FirstName: "Deleted", LastName: "Ghost"}
	b1 := &book.Book{
		XMLName:   xml.Name{Space: "", Local: ""},
		Title:     "Deleted Book",
		Author:    []book.Author{deletedAuthor},
		Lang:      "en",
		Archive:   "books.zip",
		FileName:  "del.fb2",
		FileSize:  100,
		Deleted:   true, // MARKED AS DELETED
		DateAdded: time.Now().Format("2006-01-02"),
		LibID:     1,
	}
	if err := db.Add(b1); err != nil {
		t.Fatalf("Failed to add deleted book: %v", err)
	}

	// 2. Create an author with visible books
	visibleAuthor := book.Author{FirstName: "Visible", LastName: "Writer"}
	b2 := &book.Book{
		XMLName:   xml.Name{Space: "", Local: ""},
		Title:     "Visible Book",
		Author:    []book.Author{visibleAuthor},
		Lang:      "en",
		Archive:   "books.zip",
		FileName:  "viz.fb2",
		FileSize:  100,
		Deleted:   false, // VISIBLE
		DateAdded: time.Now().Format("2006-01-02"),
		LibID:     2,
	}
	if err := db.Add(b2); err != nil {
		t.Fatalf("Failed to add visible book: %v", err)
	}

	// 3. Test GetAuthorsByLetter
	// Search for 'G' (Ghost) - should be empty
	authors, err := db.GetAuthorsByLetter("G")
	if err != nil {
		t.Fatalf("GetAuthorsByLetter failed: %v", err)
	}
	if len(authors) != 0 {
		t.Errorf("Expected 0 authors for 'G', got %d: %v", len(authors), authors)
	}

	// Search for 'W' (Writer) - should be found
	authors, err = db.GetAuthorsByLetter("W")
	if err != nil {
		t.Fatalf("GetAuthorsByLetter failed: %v", err)
	}
	if len(authors) != 1 {
		t.Errorf("Expected 1 author for 'W', got %d", len(authors))
	} else {
		if authors[0].LastName != "Writer" {
			t.Errorf("Expected author 'Writer', got '%s'", authors[0].LastName)
		}
	}

	// 4. Test GetAuthorsWithBookCountByLetter
	// Search for 'G' (Ghost) - should be empty
	authorsWithCount, err := db.GetAuthorsWithBookCountByLetter("G")
	if err != nil {
		t.Fatalf("GetAuthorsWithBookCountByLetter failed: %v", err)
	}
	if len(authorsWithCount) != 0 {
		t.Errorf("Expected 0 authors for 'G', got %d", len(authorsWithCount))
	}

	// Search for 'W' (Writer) - should have count 1
	authorsWithCount, err = db.GetAuthorsWithBookCountByLetter("W")
	if err != nil {
		t.Fatalf("GetAuthorsWithBookCountByLetter failed: %v", err)
	}
	if len(authorsWithCount) != 1 {
		t.Fatalf("Expected 1 author for 'W', got %d", len(authorsWithCount))
	}
	if authorsWithCount[0].BookCount != 1 {
		t.Errorf("Expected book count 1, got %d", authorsWithCount[0].BookCount)
	}
}
