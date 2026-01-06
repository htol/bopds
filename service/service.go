// Package service provides business logic layer between HTTP handlers and repository
package service

import (
	"context"
	"fmt"
	"io"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/repo"
)

// Service provides business logic for the application
type Service struct {
	repo            repo.Repository
	downloadService *DownloadService
}

// New creates a new Service with the given repository
func New(repo repo.Repository) *Service {
	return &Service{
		repo:            repo,
		downloadService: NewDownloadService(repo),
	}
}

// Authors

// GetAuthors retrieves all authors from the repository
func (s *Service) GetAuthors(ctx context.Context) ([]book.Author, error) {
	authors, err := s.repo.GetAuthors()
	if err != nil {
		return nil, fmt.Errorf("get authors: %w", err)
	}
	return authors, nil
}

// GetAuthorsByLetter retrieves authors whose last name starts with the given letter(s)
func (s *Service) GetAuthorsByLetter(ctx context.Context, letters string) ([]book.AuthorWithBookCount, error) {
	if letters == "" {
		return nil, fmt.Errorf("letters parameter cannot be empty")
	}
	authors, err := s.repo.GetAuthorsWithBookCountByLetter(letters)
	if err != nil {
		return nil, fmt.Errorf("get authors by letter %q: %w", letters, err)
	}
	return authors, nil
}

// GetAuthorByID retrieves a single author by ID
func (s *Service) GetAuthorByID(ctx context.Context, id int64) (*book.Author, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid author ID: %d", id)
	}
	author, err := s.repo.GetAuthorByID(id)
	if err != nil {
		return nil, fmt.Errorf("get author by ID %d: %w", id, err)
	}
	return author, nil
}

// Books

// GetBooks retrieves all books from the repository
func (s *Service) GetBooks(ctx context.Context) ([]string, error) {
	books, err := s.repo.GetBooks()
	if err != nil {
		return nil, fmt.Errorf("get books: %w", err)
	}
	return books, nil
}

// GetBooksByLetter retrieves books whose title starts with the given letter(s)
func (s *Service) GetBooksByLetter(ctx context.Context, letters string) ([]book.Book, error) {
	if letters == "" {
		return nil, fmt.Errorf("letters parameter cannot be empty")
	}
	books, err := s.repo.GetBooksByLetter(letters)
	if err != nil {
		return nil, fmt.Errorf("get books by letter %q: %w", letters, err)
	}
	return books, nil
}

// GetBooksByAuthorID retrieves books by the given author ID
func (s *Service) GetBooksByAuthorID(ctx context.Context, id int64) ([]book.Book, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid author ID: %d", id)
	}
	books, err := s.repo.GetBooksByAuthorID(id)
	if err != nil {
		return nil, fmt.Errorf("get books by author ID %d: %w", id, err)
	}
	return books, nil
}

// Genres

// GetGenres retrieves all genres from the repository
func (s *Service) GetGenres(ctx context.Context) ([]string, error) {
	genres, err := s.repo.GetGenres()
	if err != nil {
		return nil, fmt.Errorf("get genres: %w", err)
	}
	return genres, nil
}

// Write operations

// AddBook adds a new book to the repository
func (s *Service) AddBook(ctx context.Context, book *book.Book) error {
	if err := s.repo.Add(book); err != nil {
		return fmt.Errorf("add book: %w", err)
	}
	return nil
}

// Health

// Ping checks the health of the service and its dependencies
func (s *Service) Ping(ctx context.Context) error {
	if err := s.repo.Ping(); err != nil {
		return fmt.Errorf("repository ping: %w", err)
	}
	return nil
}

// Downloads

// GetBookByID retrieves a single book by ID
func (s *Service) GetBookByID(ctx context.Context, id int64) (*book.Book, error) {
	return s.downloadService.GetBookByID(ctx, id)
}

// DownloadBookFB2 returns an FB2 file stream for download
func (s *Service) DownloadBookFB2(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	return s.downloadService.DownloadBookFB2(ctx, id)
}

// DownloadBookFB2Zip returns an FB2 file packed in ZIP archive
func (s *Service) DownloadBookFB2Zip(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	return s.downloadService.DownloadBookFB2Zip(ctx, id)
}

// DownloadBookEPUB returns an EPUB file stream for download
func (s *Service) DownloadBookEPUB(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	return s.downloadService.DownloadBookEPUB(ctx, id)
}

// DownloadBookMOBI returns a MOBI file stream for download
func (s *Service) DownloadBookMOBI(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	return s.downloadService.DownloadBookMOBI(ctx, id)
}

// SearchBooks performs full-text search across books by title and/or author
// Validates query and delegates to repository with context for cancellation
func (s *Service) SearchBooks(ctx context.Context, query string, limit, offset int) ([]book.BookSearchResult, error) {
	// Validate query before calling repository
	if query == "" {
		return []book.BookSearchResult{}, nil
	}
	
	books, err := s.repo.SearchBooks(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search books: %w", err)
	}
	return books, nil
}
