package repo

import (
	"context"
	"encoding/xml"
	"testing"
	"time"

	"github.com/htol/bopds/book"
)

func TestGetBooksByAuthorID_IncludesSeries(t *testing.T) {
	dbPath := "./test_author_series.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
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

func TestGetAuthorsByLetter_FiltersDeletedBooks(t *testing.T) {
	dbPath := "./test_author_visibility.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
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

func TestGetBooksByLetter_IncludesSeries(t *testing.T) {
	dbPath := "./test_letter_series.db"
	cleanupTestDB(dbPath)
	db := GetStorage(dbPath)
	defer func() {
		db.Close()
		cleanupTestDB(dbPath)
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

func TestRepo_AddBookWithMultipleGenres(t *testing.T) {
	ctx := context.Background()

	// Setup in-memory database
	storage := GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	// Test book with multiple genres
	testBook := &book.Book{
		Title: "Test Multi-Genre Book",
		Lang:  "ru",
		Author: []book.Author{
			{FirstName: "Test", LastName: "Author"},
		},
		Genres: []string{"sf_detective", "child_sf"},
	}

	// Add book
	if err := storage.Add(testBook); err != nil {
		t.Fatalf("Failed to add book: %v", err)
	}

	// Verify book was added
	books, err := storage.GetBooks()
	if err != nil {
		t.Fatalf("Failed to get books: %v", err)
	}

	if len(books) != 1 {
		t.Errorf("Expected 1 book, got %d", len(books))
	}

	// Verify genres were added
	rows, err := storage.db.QueryContext(ctx, `
		SELECT g.name, COUNT(g.genre_id) as genre_count
		FROM books b
		JOIN book_genres bg ON b.book_id = bg.book_id
		JOIN genres g ON bg.genre_id = g.genre_id
		WHERE b.title = ?
		GROUP BY g.name
		ORDER BY g.name
	`, testBook.Title)
	if err != nil {
		t.Fatalf("Failed to query book genres: %v", err)
	}
	defer rows.Close()

	var genres []string
	for rows.Next() {
		var genre string
		var count int
		if err := rows.Scan(&genre, &count); err != nil {
			t.Fatalf("Failed to scan genre: %v", err)
		}
		genres = append(genres, genre)
		t.Logf("Genre: %s (count: %d)", genre, count)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("Error iterating genres: %v", err)
	}

	// Verify both genres are present
	expectedGenres := testBook.Genres
	if len(genres) != len(expectedGenres) {
		t.Errorf("Expected %d genres, got %d", len(expectedGenres), len(genres))
		t.Errorf("Expected genres: %v", expectedGenres)
		t.Errorf("Got genres: %v", genres)
	}

	for _, expected := range expectedGenres {
		found := false
		for _, actual := range genres {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected genre '%s' not found in result", expected)
		}
	}
}

func TestRepo_AddBookNoGenres(t *testing.T) {
	ctx := context.Background()

	storage := GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	// Test book with no genres
	testBook := &book.Book{
		Title: "Test Book Without Genres",
		Lang:  "ru",
		Author: []book.Author{
			{FirstName: "Test", LastName: "Author"},
		},
		Genres: []string{}, // No genres
	}

	// Add book - should work without genres
	if err := storage.Add(testBook); err != nil {
		t.Fatalf("Failed to add book without genres: %v", err)
	}

	// Verify book was added
	books, err := storage.GetBooks()
	if err != nil {
		t.Fatalf("Failed to get books: %v", err)
	}

	if len(books) != 1 {
		t.Errorf("Expected 1 book, got %d", len(books))
	}

	// Verify no genres were added
	var genreCount int
	err = storage.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM book_genres 
		WHERE book_id = (
			SELECT book_id FROM books WHERE title = ?
		)
	`, testBook.Title).Scan(&genreCount)

	if err != nil {
		t.Fatalf("Failed to query genre count: %v", err)
	}

	if genreCount != 0 {
		t.Errorf("Expected 0 genres, got %d", genreCount)
	}
}

func TestRepo_AddBookWithDuplicateGenres(t *testing.T) {
	ctx := context.Background()

	storage := GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	// Test book with duplicate genres in input
	testBook := &book.Book{
		Title: "Test Duplicate Genres",
		Lang:  "ru",
		Author: []book.Author{
			{FirstName: "Test", LastName: "Author"},
		},
		Genres: []string{"sf_detective", "sf_detective"}, // Duplicate
	}

	// Add book - should not create duplicate links due to PRIMARY KEY
	if err := storage.Add(testBook); err != nil {
		t.Fatalf("Failed to add book with duplicate genres: %v", err)
	}

	// Verify only 1 genre link was created (not 2)
	var genreCount int
	err := storage.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM book_genres 
		WHERE book_id = (
			SELECT book_id FROM books WHERE title = ?
		)
	`, testBook.Title).Scan(&genreCount)

	if err != nil {
		t.Fatalf("Failed to query genre count: %v", err)
	}

	if genreCount != 1 {
		t.Errorf("Expected 1 genre link (PRIMARY KEY prevented duplicate), got %d", genreCount)
	}
}

func TestRepo_AddBookWithManyGenres(t *testing.T) {
	ctx := context.Background()

	storage := GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	// Test book with many genres (like real books have)
	testBook := &book.Book{
		Title: "Test Many Genres",
		Lang:  "ru",
		Author: []book.Author{
			{FirstName: "Test", LastName: "Author"},
		},
		Genres: []string{"adv_history", "sci_history", "prose_history", "military_history", "antique"},
	}

	// Add book
	if err := storage.Add(testBook); err != nil {
		t.Fatalf("Failed to add book with many genres: %v", err)
	}

	// Verify all genres were added
	var genreCount int
	err := storage.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM book_genres 
		WHERE book_id = (
			SELECT book_id FROM books WHERE title = ?
		)
	`, testBook.Title).Scan(&genreCount)

	if err != nil {
		t.Fatalf("Failed to query genre count: %v", err)
	}

	expectedCount := len(testBook.Genres)
	if genreCount != expectedCount {
		t.Errorf("Expected %d genre links, got %d", expectedCount, genreCount)
	}
}
