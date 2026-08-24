package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/htol/bopds/book"
)

// BookCopy is one physical copy of a book: a row in one library.
type BookCopy struct {
	BookID             int64  `json:"book_id"`
	Library            string `json:"library"`
	LibraryDisplayName string `json:"library_display_name"`
	FileName           string `json:"filename,omitempty"`
	FileSize           int64  `json:"file_size,omitempty"`
	DownloadURL        string `json:"download_url"`
}

// BookGroup merges duplicate books (same author and title) across libraries
// into one card with per-library copies.
type BookGroup struct {
	Title   string           `json:"title"`
	Authors []book.Author    `json:"authors,omitempty"`
	Series  *book.SeriesInfo `json:"series,omitempty"`
	Lang    string           `json:"lang,omitempty"`
	Copies  []BookCopy       `json:"copies"`
}

// groupKey identifies duplicates across libraries: full author list + title.
func groupKey(b *book.Book) string {
	parts := make([]string, 0, len(b.Author))
	for _, a := range b.Author {
		parts = append(parts, a.LastName, a.FirstName, a.MiddleName)
	}
	return strings.Join(parts, "\x00") + "\x00\x00" + b.Title
}

// groupBooks merges books with the same author+title into groups; rows that
// are alone stay groups of one. Input order is preserved (first occurrence).
func groupBooks(books []book.Book) []BookGroup {
	groups := make([]BookGroup, 0, len(books))
	indexByKey := make(map[string]int, len(books))

	for i := range books {
		b := &books[i]
		key := groupKey(b)

		if gi, ok := indexByKey[key]; ok {
			groups[gi].Copies = append(groups[gi].Copies, newCopy(b))
			continue
		}

		indexByKey[key] = len(groups)
		groups = append(groups, BookGroup{
			Title:   b.Title,
			Authors: b.Author,
			Series:  b.Series,
			Lang:    b.Lang,
			Copies:  []BookCopy{newCopy(b)},
		})
	}

	return groups
}

func newCopy(b *book.Book) BookCopy {
	displayName := b.LibraryDisplayName
	if displayName == "" {
		displayName = b.Library
	}
	return BookCopy{
		BookID:             b.BookID,
		Library:            b.Library,
		LibraryDisplayName: displayName,
		FileName:           b.FileName,
		FileSize:           b.FileSize,
		DownloadURL:        fmt.Sprintf("/api/books/%d/download?format=fb2", b.BookID),
	}
}

// GetBooksByLetterGrouped lists books by title prefix with duplicates grouped.
func (s *Service) GetBooksByLetterGrouped(ctx context.Context, letters string) ([]BookGroup, error) {
	books, err := s.GetBooksByLetter(ctx, letters)
	if err != nil {
		return nil, err
	}
	return groupBooks(books), nil
}

// GetBooksByAuthorIDGrouped lists an author's books with duplicates grouped.
func (s *Service) GetBooksByAuthorIDGrouped(ctx context.Context, id int64) ([]BookGroup, error) {
	books, err := s.GetBooksByAuthorID(ctx, id)
	if err != nil {
		return nil, err
	}
	return groupBooks(books), nil
}
