package repo

import (
	"context"
	"errors"
	"github.com/htol/bopds/book"
)

// ErrNotFound is returned when a record is not found in the repository
var ErrNotFound = errors.New("record not found")

// Repository defines the interface for data access operations
type Repository interface {
	// Close closes the database connection
	Close() error

	// Health check
	Ping() error

	// Authors
	GetAuthors() ([]book.Author, error)
	GetAuthorsByLetter(letters string) ([]book.Author, error)
	GetAuthorByID(id int64) (*book.Author, error)
	GetAuthorsWithBookCount() ([]book.AuthorWithBookCount, error)
	GetAuthorsWithBookCountByLetter(letters string) ([]book.AuthorWithBookCount, error)

	// Books
	GetBooks() ([]string, error)
	GetBooksByLetter(letters string) ([]book.Book, error)
	GetBooksByAuthorID(id int64) ([]book.Book, error)
	GetBookByID(id int64) (*book.Book, error)

	// SearchBooks performs full-text search across books by title and author
	// Returns results ranked by relevance (FTS5 rank)
	SearchBooks(ctx context.Context, query string, limit, offset int) ([]book.BookSearchResult, error)

	// Genres
	GetGenres() ([]string, error)

	// Write operations
	Add(record *book.Book) error
	Search() error
	List() error
}
