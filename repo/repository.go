package repo

import (
	"github.com/htol/bopds/book"
)

// Repository defines the interface for data access operations
type Repository interface {
	// Close closes the database connection
	Close() error

	// Authors
	GetAuthors() ([]book.Author, error)
	GetAuthorsByLetter(letters string) ([]book.Author, error)

	// Books
	GetBooks() ([]string, error)
	GetBooksByLetter(letters string) ([]book.Book, error)
	GetBooksByAuthorID(id int64) ([]string, error)

	// Write operations
	Add(record *book.Book) error
	Search() error
	List() error
}
