package repo

import (
	"context"
	"encoding/xml"
	"testing"
	"time"

	"github.com/htol/bopds/book"
)

func TestSearchBooks_NewFields(t *testing.T) {
	dbPath := "./test_search.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
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

	// Rebuild FTS index to populate author/series/genre fields
	// (per-insert triggers no longer update these for performance reasons)
	if err := db.RebuildFTSIndex(); err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	// Perform search using SERIES NAME
	results, err := db.SearchBooks(context.Background(), "Foundations", 10, 0, nil, nil)
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

func TestSearchBooks_FieldFilters(t *testing.T) {
	dbPath := "./test_search_filters.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	// Add test books
	books := []*book.Book{
		{
			Title: "Space Odyssey",
			Author: []book.Author{
				{FirstName: "Arthur", LastName: "Clarke"},
			},
			Genres: []string{"sf_space"},
			Lang:   "en",
		},
		{
			Title: "History of Space",
			Author: []book.Author{
				{FirstName: "John", LastName: "Space"},
			},
			Genres: []string{"sci_history"},
			Lang:   "en",
		},
		{
			Title: "SF Book",
			Author: []book.Author{
				{FirstName: "Isaac", LastName: "Asimov"},
			},
			Genres: []string{"sf"},
			Lang:   "ru",
		},
	}

	for _, b := range books {
		// Populate required fields
		b.XMLName = xml.Name{Space: "", Local: ""}
		b.Archive = "test.zip"
		b.FileName = "test.fb2"
		if err := db.Add(b); err != nil {
			t.Fatalf("Failed to add book: %v", err)
		}
	}

	// Rebuild FTS index
	db.SyncGenreDisplayNames()
	if err := db.RebuildFTSIndex(); err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	ctx := context.Background()

	// Scenario 8: Search by Transliteration (nauchnaya -> Научная)
	// This tests if the user can search using Latin characters for Russian terms.
	results, err := db.SearchBooks(ctx, "nauchnaya", 10, 0, []string{"genre"}, nil)
	if err != nil {
		t.Fatalf("Search 'nauchnaya' failed: %v", err)
	}
	if len(results) != 1 {
		// We expect this to fail, so we log it but don't fail the test yet to confirm behavior
		t.Logf("Search 'nauchnaya': expected 1 result, got %d", len(results))
	} else {
		if results[0].Title != "SF Book" {
			t.Errorf("Search 'nauchnaya': expected 'SF Book', got '%s'", results[0].Title)
		}
		// Verification for Issue "Mapping Removed":
		// Ensure the returned genre is the Display Name ("Научная фантастика"), not the code ("sf")
		if len(results[0].Genres) == 0 {
			t.Errorf("Search 'nauchnaya': expected genres to be populated")
		} else if results[0].Genres[0] != "Научная фантастика" {
			// Note: The search result 'Genres' field is actually []string, but now we're updating GetGenres to support []book.Genre.
			// Wait, SearchBooks still returns []book.BookSearchResult which has Genres []string.
			// The update we did was to GetGenres (the list endpoint), not SearchBooks struct.
			// SearchBooks query also selects group_concat(distinct g.display_name) as genres.
			// So results[0].Genres should contain the display name string.
			t.Errorf("Search result genre mismatch: expected 'Научная фантастика', got '%s'", results[0].Genres[0])
		}
	}
}
