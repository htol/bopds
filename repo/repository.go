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
	GetRecentBooks(limit, offset int) ([]book.Book, int, error)
	GetBooksByGenre(genre string, limit, offset int) ([]book.Book, int, error)

	// SearchBooks performs full-text search across books by title and author
	// Returns results ranked by relevance (FTS5 rank)
	SearchBooks(ctx context.Context, query string, limit, offset int, fields []string, languages []string) ([]book.BookSearchResult, error)

	// Genres
	GetGenres() ([]book.Genre, error)

	// Languages
	GetLanguages() ([]string, error)

	// Write operations
	Add(record *book.Book) error
	Search() error
	List() error

	// RebuildFTSIndex rebuilds the full-text search index for all books
	RebuildFTSIndex() error
	CreateIndexes() error
	DropIndexes() error
	InitCache() error
	SetFastMode(enable bool) error

	// Bulk import optimizations
	SetBulkImportMode(enable bool) error
	CheckpointWAL() error
}
