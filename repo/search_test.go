package repo

import (
	"context"
	"encoding/xml"
	"os"
	"testing"
	"time"

	"github.com/htol/bopds/book"
)

func TestSearchBooks_NewFields(t *testing.T) {
	dbPath := "./test_search.db"
	os.Remove(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		os.Remove(dbPath)
	}()

	// Create test series
	seriesName := "Foundations of Math"
	seriesNo := 1

	// Create test book with all fields
	b := &book.Book{
		XMLName:  xml.Name{Space: "", Local: ""},
		Title:    "Advanced Calculus",
		Author:   []book.Author{{FirstName: "John", LastName: "Doe"}},
		Lang:     "en",
		Archive:  "books.zip",
		FileName: "calc.fb2",
		FileSize: 1024567,
		Deleted:  false,
		Series: &book.SeriesInfo{
			Name:     seriesName,
			SeriesNo: seriesNo,
		},
		DateAdded: time.Now().Format("2006-01-02"),
		LibID:     1,
	}

	if err := db.Add(b); err != nil {
		t.Fatalf("Failed to add book: %v", err)
	}

	// Rebuild FTS index if necessary (triggers should handle it, but triggers update author field)
	// We need to make sure FTS is consistent.
	// The Add function uses a transaction and triggers, so it should be fine.

	// Perform search using SERIES NAME
	results, err := db.SearchBooks(context.Background(), "Foundations", 10, 0)
	if err != nil {
		t.Fatalf("SearchBooks failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]

	// Verify new fields
	if res.FileSize != 1024567 {
		t.Errorf("Expected FileSize 1024567, got %d", res.FileSize)
	}
	if res.SeriesName != seriesName {
		t.Errorf("Expected SeriesName %q, got %q", seriesName, res.SeriesName)
	}
	if res.SeriesNo != seriesNo {
		t.Errorf("Expected SeriesNo %d, got %d", seriesNo, res.SeriesNo)
	}
	if res.Deleted {
		t.Error("Expected Deleted to be false")
	}
}
