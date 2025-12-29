package repo

import (
	"context"
	"testing"

	"github.com/htol/bopds/book"
)

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

	t.Logf("✅ Successfully added book '%s' with %d genres: %v",
		testBook.Title, len(genres), genres)
}
