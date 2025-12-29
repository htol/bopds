package repo

import (
	"context"
	"testing"

	"github.com/htol/bopds/book"
)

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

	t.Logf("✅ Successfully added book without genres")
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

	t.Logf("✅ PRIMARY KEY prevented duplicate genre links (count: %d)", genreCount)
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

	t.Logf("✅ Successfully added book with %d genres", genreCount)
}
