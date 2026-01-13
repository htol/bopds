package repo

import (
	"encoding/xml"
	"os"
	"testing"
	"time"

	"github.com/htol/bopds/book"
)

func TestGetBooksByLetter_IncludesSeries(t *testing.T) {
	dbPath := "./test_letter_series.db"
	os.Remove(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		os.Remove(dbPath)
	}()

	// Setup data
	seriesName := "The Letter Series"
	seriesNo := 1

	// Create book starting with 'A'
	b := &book.Book{
		XMLName:  xml.Name{Space: "", Local: ""},
		Title:    "A Great Book",
		Author:   []book.Author{{FirstName: "John", LastName: "Doe"}},
		Lang:     "en",
		Archive:  "books.zip",
		FileName: "a_book.fb2",
		FileSize: 500000,
		Series: &book.SeriesInfo{
			Name:     seriesName,
			SeriesNo: seriesNo,
		},
		DateAdded: time.Now().Format("2006-01-02"),
		LibID:     1,
	}

	// Add book
	if err := db.Add(b); err != nil {
		t.Fatalf("Failed to add book: %v", err)
	}

	// Fetch books by letter 'A'
	books, err := db.GetBooksByLetter("A")
	if err != nil {
		t.Fatalf("GetBooksByLetter failed: %v", err)
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
