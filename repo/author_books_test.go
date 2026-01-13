package repo

import (
	"encoding/xml"
	"os"
	"testing"
	"time"

	"github.com/htol/bopds/book"
)

func TestGetBooksByAuthorID_IncludesSeries(t *testing.T) {
	dbPath := "./test_author_series.db"
	os.Remove(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		os.Remove(dbPath)
	}()

	// Setup data
	seriesName := "The Foundation"
	seriesNo := 1
	author := book.Author{
		FirstName: "Isaac",
		LastName:  "Asimov",
	}

	b := &book.Book{
		XMLName:  xml.Name{Space: "", Local: ""},
		Title:    "Foundation",
		Author:   []book.Author{author},
		Lang:     "en",
		Archive:  "books.zip",
		FileName: "foundation.fb2",
		FileSize: 500000,
		Series: &book.SeriesInfo{
			Name:     seriesName,
			SeriesNo: seriesNo,
		},
		DateAdded: time.Now().Format("2006-01-02"),
		LibID:     1,
	}

	// Add book (this creates author and series)
	if err := db.Add(b); err != nil {
		t.Fatalf("Failed to add book: %v", err)
	}

	// Get author ID (should be 1)
	authors, err := db.GetAuthors()
	if err != nil {
		t.Fatalf("Failed to get authors: %v", err)
	}
	if len(authors) != 1 {
		t.Fatalf("Expected 1 author, got %d", len(authors))
	}
	authorID := authors[0].ID

	// Fetch books by author
	books, err := db.GetBooksByAuthorID(authorID)
	if err != nil {
		t.Fatalf("GetBooksByAuthorID failed: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(books))
	}

	fetchedBook := books[0]
	if fetchedBook.Series == nil {
		t.Fatal("Expected Series to be populated, got nil")
	}

	if fetchedBook.Series.Name != seriesName {
		t.Errorf("Expected SeriesName %q, got %q", seriesName, fetchedBook.Series.Name)
	}
	if fetchedBook.Series.SeriesNo != seriesNo {
		t.Errorf("Expected SeriesNo %d, got %d", seriesNo, fetchedBook.Series.SeriesNo)
	}
}
