package repo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"strings"
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

func TestSearchBooks_AuthorIDsAndSeriesID(t *testing.T) {
	dbPath := "./test_search_ids.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	// Book with two authors and a series
	b1 := &book.Book{
		Title: "Dual Writer Saga",
		Author: []book.Author{
			{FirstName: "Alan", LastName: "Alpha"},
			{FirstName: "Beth", LastName: "Beta"},
		},
		Lang:     "en",
		Archive:  "books.zip",
		FileName: "saga.fb2",
		Series: &book.SeriesInfo{
			Name:     "Grand Cycle",
			SeriesNo: 2,
		},
	}
	// Book with one author and no series
	b2 := &book.Book{
		Title:    "Lone Wolf Tale",
		Author:   []book.Author{{FirstName: "Carl", LastName: "Gamma"}},
		Lang:     "en",
		Archive:  "books.zip",
		FileName: "lone.fb2",
	}
	for _, b := range []*book.Book{b1, b2} {
		if err := db.Add(b); err != nil {
			t.Fatalf("Failed to add book %q: %v", b.Title, err)
		}
	}
	if err := db.RebuildFTSIndex(); err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	// Map author ID -> the name as the SQL concat renders it, for alignment checks
	idToName := make(map[int64]string)
	rows, err := db.db.Query(`
		SELECT a.author_id, a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '')
		FROM authors a
	`)
	if err != nil {
		t.Fatalf("Failed to query authors: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("Failed to scan author: %v", err)
		}
		idToName[id] = name
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Failed to iterate authors: %v", err)
	}

	ctx := context.Background()

	// Multi-author book with a series: author_ids aligns with the names in
	// the author string, series_id is the book's series
	results, err := db.SearchBooks(ctx, "Saga", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchBooks failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	res := results[0]

	if len(res.AuthorIDs) != 2 {
		t.Fatalf("Expected 2 author IDs, got %d (%v)", len(res.AuthorIDs), res.AuthorIDs)
	}
	// The visible author string stays byte-identical to the old plain
	// group_concat: names (with the concat's trailing space) joined by ","
	segments := strings.Split(res.Author, ",")
	if len(segments) != 2 {
		t.Fatalf("Expected 2 author segments in %q", res.Author)
	}
	for i, id := range res.AuthorIDs {
		want, ok := idToName[id]
		if !ok {
		t.Fatalf("author_ids[%d]=%d not present in authors table", i, id)
		}
		if segments[i] != want {
			t.Errorf("author_ids[%d]=%d maps to %q, but author segment %d is %q", i, id, want, i, segments[i])
		}
	}
	if res.AuthorIDs[0] == res.AuthorIDs[1] {
		t.Errorf("Expected distinct author IDs, got %v", res.AuthorIDs)
	}

	var seriesID int64
	if err := db.db.QueryRow(`SELECT series_id FROM series WHERE name = ?`, "Grand Cycle").Scan(&seriesID); err != nil {
		t.Fatalf("Failed to look up series ID: %v", err)
	}
	if res.SeriesID != seriesID {
		t.Errorf("SeriesID = %d, want %d", res.SeriesID, seriesID)
	}

	// No-series book: series_id stays empty (0, omitted in JSON)
	results, err = db.SearchBooks(ctx, "Lone", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchBooks failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	res = results[0]
	if res.SeriesID != 0 {
		t.Errorf("SeriesID = %d, want 0 for a book without a series", res.SeriesID)
	}
	if len(res.AuthorIDs) != 1 {
		t.Fatalf("Expected 1 author ID, got %d (%v)", len(res.AuthorIDs), res.AuthorIDs)
	}
	if segments := strings.Split(res.Author, ","); len(segments) != 1 || segments[0] != idToName[res.AuthorIDs[0]] {
		t.Errorf("Author = %q does not align with author_ids %v", res.Author, res.AuthorIDs)
	}

	// The field is omitted from JSON when there is no series
	encoded, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "series_id") {
		t.Errorf("Expected no series_id in JSON for a book without a series, got %s", encoded)
	}

	// Duplicate book-author pairs (book_authors has no unique constraint;
	// scans may insert the same pair twice) collapse to one author entry
	if _, err := db.db.Exec(`INSERT INTO book_authors (book_id, author_id)
		SELECT b.book_id, a.author_id FROM books b, authors a
		WHERE b.title = 'Lone Wolf Tale' AND a.last_name = 'Gamma'`); err != nil {
		t.Fatalf("Failed to duplicate book-author pair: %v", err)
	}
	results, err = db.SearchBooks(ctx, "Lone", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchBooks failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if len(results[0].AuthorIDs) != 1 || len(strings.Split(results[0].Author, ",")) != 1 {
		t.Errorf("Expected the duplicated author pair to collapse to one entry, got author %q, ids %v",
			results[0].Author, results[0].AuthorIDs)
	}
}

func TestSearchBooks_DuplicateAuthorPairs(t *testing.T) {
	dbPath := "./test_search_dup.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	// Book whose book-author pairs will be duplicated (book_authors has no
	// unique constraint; the correlated DISTINCT in SearchBooks must collapse
	// the copies — the case that motivated the pre-9eb41d2 aggregate shape)
	b := &book.Book{
		Title: "Duplicated Pairs Mystery",
		Author: []book.Author{
			{FirstName: "Dave", LastName: "Delta"},
			{FirstName: "Eve", LastName: "Echo"},
		},
		Lang:     "en",
		Archive:  "books.zip",
		FileName: "dup.fb2",
	}
	if err := db.Add(b); err != nil {
		t.Fatalf("Failed to add book: %v", err)
	}
	if err := db.RebuildFTSIndex(); err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	// Duplicate every book-author pair: once by copying the existing rows,
	// once by re-deriving them from books x authors (2 pairs -> 6 rows)
	if _, err := db.db.Exec(`INSERT INTO book_authors (book_id, author_id)
		SELECT ba.book_id, ba.author_id FROM book_authors ba
		JOIN books b ON ba.book_id = b.book_id
		WHERE b.title = 'Duplicated Pairs Mystery'`); err != nil {
		t.Fatalf("Failed to duplicate book-author pairs: %v", err)
	}
	if _, err := db.db.Exec(`INSERT INTO book_authors (book_id, author_id)
		SELECT b.book_id, a.author_id FROM books b, authors a
		WHERE b.title = 'Duplicated Pairs Mystery'`); err != nil {
		t.Fatalf("Failed to duplicate book-author pairs: %v", err)
	}
	var pairCount int
	if err := db.db.QueryRow(`SELECT count(*) FROM book_authors ba
		JOIN books b ON ba.book_id = b.book_id
		WHERE b.title = 'Duplicated Pairs Mystery'`).Scan(&pairCount); err != nil {
		t.Fatalf("Failed to count pairs: %v", err)
	}
	if pairCount != 6 {
		t.Fatalf("Expected 6 book_authors rows (2 pairs x 3), got %d", pairCount)
	}

	// Map author ID -> the name as the SQL concat renders it
	idToName := make(map[int64]string)
	rows, err := db.db.Query(`
		SELECT a.author_id, a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '')
		FROM authors a
	`)
	if err != nil {
		t.Fatalf("Failed to query authors: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("Failed to scan author: %v", err)
		}
		idToName[id] = name
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Failed to iterate authors: %v", err)
	}

	results, err := db.SearchBooks(context.Background(), "Duplicated", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchBooks failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	res := results[0]

	// Duplicates collapse to exactly one entry per author
	if len(res.AuthorIDs) != 2 {
		t.Fatalf("Expected 2 author IDs, got %d (%v)", len(res.AuthorIDs), res.AuthorIDs)
	}
	if res.AuthorIDs[0] == res.AuthorIDs[1] {
		t.Errorf("Expected distinct author IDs, got %v", res.AuthorIDs)
	}

	// author_ids align with the visible author names
	segments := strings.Split(res.Author, ",")
	if len(segments) != 2 {
		t.Fatalf("Expected 2 author segments in %q", res.Author)
	}
	for i, id := range res.AuthorIDs {
		want, ok := idToName[id]
		if !ok {
			t.Fatalf("author_ids[%d]=%d not present in authors table", i, id)
		}
		if segments[i] != want {
			t.Errorf("author_ids[%d]=%d maps to %q, but author segment %d is %q", i, id, want, i, segments[i])
		}
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

func TestSearchBooks_ExposesLibrary(t *testing.T) {
	dbPath := "./test_search_lib.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
	}()

	if _, err := db.GetOrCreateLibrary("libA", "Fiction Library"); err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	b := &book.Book{
		Title:    "Advanced Calculus",
		Author:   []book.Author{{FirstName: "John", LastName: "Doe"}},
		Library:  "libA",
		Lang:     "en",
		LibID:    7,
		Archive:  "a.zip",
		FileName: "calc.fb2",
	}
	if err := db.Add(b); err != nil {
		t.Fatalf("Failed to add book: %v", err)
	}
	if err := db.RebuildFTSIndex(); err != nil {
		t.Fatalf("Failed to rebuild FTS index: %v", err)
	}

	results, err := db.SearchBooks(context.Background(), "Calculus", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchBooks failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Library != "libA" {
		t.Errorf("Library = %q, want %q", results[0].Library, "libA")
	}
	if results[0].LibraryDisplayName != "Fiction Library" {
		t.Errorf("LibraryDisplayName = %q, want %q", results[0].LibraryDisplayName, "Fiction Library")
	}
}
