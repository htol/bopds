package service

import (
	"archive/zip"
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

// DownloadBookFB2 returns an unpacked FB2 file stream
func (s *DownloadService) DownloadBookFB2(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	// Get book info
	b, err := s.GetBookByID(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// Extract FB2 from archive (ZIP or 7z)
	reader, err := s.converter.ExtractFromArchive(b.Archive, b.FileName)
	if err != nil {
		return nil, "", fmt.Errorf("extract FB2 from archive: %w", err)
	}

	// Generate filename as "Author - Title.fb2"
	filename := converter.FormatBookFilename(b, "fb2")

	return reader, filename, nil
}

// DownloadBookFB2Zip returns an FB2 file packed in ZIP archive
func (s *DownloadService) DownloadBookFB2Zip(ctx context.Context, id int64) (io.ReadCloser, string, error) {
	// Get book info
	b, err := s.GetBookByID(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// Extract FB2 from archive (ZIP or 7z)
	fb2Reader, err := s.converter.ExtractFromArchive(b.Archive, b.FileName)
	if err != nil {
		return nil, "", fmt.Errorf("extract FB2 from archive: %w", err)
	}
	defer fb2Reader.Close()

	// Read FB2 content
	fb2Data, err := io.ReadAll(fb2Reader)
	if err != nil {
		return nil, "", fmt.Errorf("read FB2 content: %w", err)
	}

	// Create a temporary file for the ZIP archive
	tempFile, err := os.CreateTemp("", "fb2-*.zip")
	if err != nil {
		return nil, "", fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Generate filename for archive: "Author - Title.fb2"
	archiveFilename := converter.FormatBookFilename(b, "fb2")

	// Create ZIP archive
	zipWriter := zip.NewWriter(tempFile)

	// Create ZIP entry with the same name as archive (without .zip extension)
	writer, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   archiveFilename, // Filename inside archive matches archive name
		Method: zip.Deflate,     // Standard compression
		Flags:  0x800,           // UTF-8 flag for proper encoding
	})
	if err != nil {
		zipWriter.Close()
		tempFile.Close()
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("create ZIP entry: %w", err)
	}

	// Write FB2 content to ZIP
	if _, err := writer.Write(fb2Data); err != nil {
		zipWriter.Close()
		tempFile.Close()
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("write FB2 to ZIP: %w", err)
	}

	// Close ZIP writer
	if err := zipWriter.Close(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("close ZIP writer: %w", err)
	}

	// Reopen temp file for reading
	if _, err := tempFile.Seek(0, 0); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("seek temp file: %w", err)
	}

	// Return with .fb2.zip extension
	filename := converter.FormatBookFilename(b, "fb2.zip")

	// Return cleanupReadCloser that removes temp file when done
	return &cleanupReadCloser{
		ReadCloser: tempFile,
		cleanup: func() {
			os.Remove(tempPath)
		},
	}, filename, nil
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

	// Extract from archive and write to temp file
	reader, err := s.converter.ExtractFromArchive(b.Archive, b.FileName)
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

// DownloadBookMOBI returns a MOBI file stream (converts on-the-fly)
func (s *DownloadService) DownloadBookMOBI(ctx context.Context, id int64) (io.ReadCloser, string, error) {
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

	// Extract from archive and write to temp file
	reader, err := s.converter.ExtractFromArchive(b.Archive, b.FileName)
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

	// Convert to MOBI
	mobiReader, _, err := s.converter.ConvertFB2ToMOBI(ctx, tempPath)
	if err != nil {
		os.Remove(tempPath)
		return nil, "", fmt.Errorf("convert FB2 to MOBI: %w", err)
	}

	// Clean up temp FB2 file
	os.Remove(tempPath)

	// Generate filename as "Author - Title.mobi"
	filename := converter.FormatBookFilename(b, "mobi")

	return mobiReader, filename, nil
}

// cleanupReadCloser wraps a ReadCloser and calls cleanup on close
type cleanupReadCloser struct {
	io.ReadCloser
	cleanup func()
}

func (rc *cleanupReadCloser) Close() error {
	var err1 error
	if rc.ReadCloser != nil {
		err1 = rc.ReadCloser.Close()
	}
	if rc.cleanup != nil {
		rc.cleanup()
	}
	return err1
}
