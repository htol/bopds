// Package service provides business logic layer between HTTP handlers and repository
package service

import (
	"context"
	"fmt"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/repo"
)

// Service provides business logic for the application
type Service struct {
	repo repo.Repository
}

// New creates a new Service with the given repository
func New(repo repo.Repository) *Service {
	return &Service{
		repo: repo,
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
func (s *Service) GetAuthorsByLetter(ctx context.Context, letters string) ([]book.Author, error) {
	if letters == "" {
		return nil, fmt.Errorf("letters parameter cannot be empty")
	}
	authors, err := s.repo.GetAuthorsByLetter(letters)
	if err != nil {
		return nil, fmt.Errorf("get authors by letter %q: %w", letters, err)
	}
	return authors, nil
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
func (s *Service) GetBooksByAuthorID(ctx context.Context, id int64) ([]string, error) {
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
