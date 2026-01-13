package service

import (
	"context"
	"testing"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
)

func init() {
	// Initialize logger for tests
	logger.Init("info")
}

// Mock repository for testing
type mockRepository struct {
	authors      []book.Author
	authorsError error
	books        []string
	booksError   error
	genres       []string
	genresError  error
	pingError    error
}

func (m *mockRepository) Close() error {
	return nil
}

func (m *mockRepository) Ping() error {
	return m.pingError
}

func (m *mockRepository) GetAuthors() ([]book.Author, error) {
	if m.authorsError != nil {
		return nil, m.authorsError
	}
	return m.authors, nil
}

func (m *mockRepository) GetAuthorsByLetter(letters string) ([]book.Author, error) {
	if m.authorsError != nil {
		return nil, m.authorsError
	}
	// Filter authors by last name starting with letters
	var result []book.Author
	for _, author := range m.authors {
		if len(author.LastName) >= len(letters) {
			if author.LastName[:len(letters)] == letters {
				result = append(result, author)
			}
		}
	}
	return result, nil
}

func (m *mockRepository) GetAuthorByID(id int64) (*book.Author, error) {
	if m.authorsError != nil {
		return nil, m.authorsError
	}
	for _, author := range m.authors {
		if author.ID == id {
			return &author, nil
		}
	}
	return nil, &testError{msg: "author not found"}
}

func (m *mockRepository) GetAuthorsWithBookCount() ([]book.AuthorWithBookCount, error) {
	if m.authorsError != nil {
		return nil, m.authorsError
	}
	var result []book.AuthorWithBookCount
	for _, author := range m.authors {
		result = append(result, book.AuthorWithBookCount{
			ID:         author.ID,
			FirstName:  author.FirstName,
			MiddleName: author.MiddleName,
			LastName:   author.LastName,
			BookCount:  0,
		})
	}
	return result, nil
}

func (m *mockRepository) GetAuthorsWithBookCountByLetter(letters string) ([]book.AuthorWithBookCount, error) {
	if m.authorsError != nil {
		return nil, m.authorsError
	}
	var result []book.AuthorWithBookCount
	for _, author := range m.authors {
		if len(author.LastName) >= len(letters) {
			if author.LastName[:len(letters)] == letters {
				result = append(result, book.AuthorWithBookCount{
					ID:         author.ID,
					FirstName:  author.FirstName,
					MiddleName: author.MiddleName,
					LastName:   author.LastName,
					BookCount:  0,
				})
			}
		}
	}
	return result, nil
}

func (m *mockRepository) GetBooks() ([]string, error) {
	if m.booksError != nil {
		return nil, m.booksError
	}
	return m.books, nil
}

func (m *mockRepository) GetBooksByLetter(letters string) ([]book.Book, error) {
	if m.booksError != nil {
		return nil, m.booksError
	}
	return []book.Book{}, nil
}

func (m *mockRepository) GetBooksByAuthorID(id int64) ([]book.Book, error) {
	if m.booksError != nil {
		return nil, m.booksError
	}
	return []book.Book{}, nil
}

func (m *mockRepository) GetBookByID(id int64) (*book.Book, error) {
	if m.booksError != nil {
		return nil, m.booksError
	}
	return nil, &testError{msg: "book not found"}
}

func (m *mockRepository) GetGenres() ([]string, error) {
	if m.genresError != nil {
		return nil, m.genresError
	}
	return m.genres, nil
}

func (m *mockRepository) Add(b *book.Book) error {
	return nil
}

func (m *mockRepository) Search() error {
	return nil
}

func (m *mockRepository) List() error {
	return nil
}

func (m *mockRepository) SearchBooks(ctx context.Context, query string, limit, offset int) ([]book.BookSearchResult, error) {
	return []book.BookSearchResult{}, nil
}

func (m *mockRepository) RebuildFTSIndex() error {
	return nil
}

func (m *mockRepository) CreateIndexes() error {
	return nil
}

func (m *mockRepository) DropIndexes() error {
	return nil
}

func (m *mockRepository) InitCache() error {
	return nil
}

func (m *mockRepository) SetFastMode(enable bool) error {
	return nil
}

func TestService_GetAuthors(t *testing.T) {
	tests := []struct {
		name        string
		authors     []book.Author
		authorsErr  error
		expectError bool
		expectCount int
	}{
		{
			name: "success with authors",
			authors: []book.Author{
				{FirstName: "Test", LastName: "Author"},
				{FirstName: "Another", LastName: "Writer"},
			},
			authorsErr:  nil,
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "empty list",
			authors:     []book.Author{},
			authorsErr:  nil,
			expectError: false,
			expectCount: 0,
		},
		{
			name:        "repository error",
			authors:     nil,
			authorsErr:  &testError{msg: "database error"},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockRepository{
				authors:      tt.authors,
				authorsError: tt.authorsErr,
			}
			svc := New(mockRepo)

			ctx := context.Background()
			authors, err := svc.GetAuthors(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(authors) != tt.expectCount {
				t.Errorf("Expected %d authors, got %d", tt.expectCount, len(authors))
			}
		})
	}
}

func TestService_GetAuthorsByLetter_EmptyString(t *testing.T) {
	mockRepo := &mockRepository{
		authors: []book.Author{
			{FirstName: "Test", LastName: "Author"},
		},
	}
	svc := New(mockRepo)

	ctx := context.Background()
	_, err := svc.GetAuthorsByLetter(ctx, "")

	if err == nil {
		t.Errorf("Expected error for empty letters parameter")
	}

	if err.Error() != "letters parameter cannot be empty" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestService_GetGenres(t *testing.T) {
	tests := []struct {
		name        string
		genres      []string
		genresErr   error
		expectError bool
		expectCount int
	}{
		{
			name:        "success with genres",
			genres:      []string{"Fiction", "Science", "History"},
			genresErr:   nil,
			expectError: false,
			expectCount: 3,
		},
		{
			name:        "empty list",
			genres:      []string{},
			genresErr:   nil,
			expectError: false,
			expectCount: 0,
		},
		{
			name:        "repository error",
			genres:      nil,
			genresErr:   &testError{msg: "database error"},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockRepository{
				genres:      tt.genres,
				genresError: tt.genresErr,
			}
			svc := New(mockRepo)

			ctx := context.Background()
			genres, err := svc.GetGenres(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(genres) != tt.expectCount {
				t.Errorf("Expected %d genres, got %d", tt.expectCount, len(genres))
			}
		})
	}
}

func TestService_Ping(t *testing.T) {
	tests := []struct {
		name        string
		pingError   error
		expectError bool
	}{
		{
			name:        "success",
			pingError:   nil,
			expectError: false,
		},
		{
			name:        "failure",
			pingError:   &testError{msg: "connection failed"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockRepository{
				pingError: tt.pingError,
			}
			svc := New(mockRepo)

			ctx := context.Background()
			err := svc.Ping(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestService_AddBook(t *testing.T) {
	mockRepo := &mockRepository{}
	svc := New(mockRepo)

	ctx := context.Background()
	testBook := &book.Book{
		Title: "Test Book",
		Lang:  "en",
	}

	err := svc.AddBook(ctx, testBook)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
