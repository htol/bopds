package service

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/converter"
	"github.com/htol/bopds/repo"
)

// DownloadService handles book download operations
type DownloadService struct {
	repo      repo.Repository
	converter *converter.Converter
}

// NewDownloadService creates a new download service
func NewDownloadService(r repo.Repository) *DownloadService {
	return &DownloadService{
		repo:      r,
		converter: converter.New(),
	}
}

// GetBookByID retrieves a single book by ID
func (s *DownloadService) GetBookByID(ctx context.Context, id int64) (*book.Book, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid book ID: must be positive")
	}

	b, err := s.repo.GetBookByID(id)
	if err != nil {
		return nil, fmt.Errorf("get book by ID %d: %w", id, err)
	}

	return b, nil
}

// DownloadBookFB2 returns an FB2 file stream
func (s *DownloadService) DownloadBookFB2(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	// Get book info
	b, err := s.GetBookByID(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// Extract FB2 from ZIP
	reader, err := s.converter.ExtractFromZIP(b.Archive, b.FileName)
	if err != nil {
		return nil, "", fmt.Errorf("extract FB2 from archive: %w", err)
	}

	// Generate filename as "Author - Title.fb2"
	filename := converter.FormatBookFilename(b, "fb2")

	return reader, filename, nil
}

// DownloadBookEPUB returns an EPUB file stream (converts on-the-fly)
func (s *DownloadService) DownloadBookEPUB(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	// Get book info
	b, err := s.GetBookByID(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// Extract FB2 to temporary file
	tempFile, err := os.CreateTemp("", "fb2-*.fb2")
	if err != nil {
		return nil, "", fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Extract from ZIP and write to temp file
	reader, err := s.converter.ExtractFromZIP(b.Archive, b.FileName)
	if err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("extract FB2 from archive: %w", err)
	}

	// Copy FB2 content to temp file
	if _, err := io.Copy(tempFile, reader); err != nil {
		reader.Close()
		tempFile.Close()
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("write FB2 to temp file: %w", err)
	}
	reader.Close()
	tempFile.Close()

	// Reopen temp file for reading
	epubReader, _, err := s.converter.ConvertFB2ToEPUB(ctx, tempPath)
	if err != nil {
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("convert FB2 to EPUB: %w", err)
	}

	// Clean up temp FB2 file
	os.Remove(tempPath)

	// Generate filename as "Author - Title.epub"
	filename := converter.FormatBookFilename(b, "epub")

	return epubReader, filename, nil
}
