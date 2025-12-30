package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/htol/bopds/book"
	"github.com/vinser/fb2epub/converter"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// Converter handles FB2 extraction and EPUB conversions
type Converter struct{}

// New creates a new Converter
func New() *Converter {
	return &Converter{}
}

// ExtractFromZIP extracts an FB2 file from a ZIP archive
func (c *Converter) ExtractFromZIP(archivePath, filename string) (io.ReadCloser, error) {
	// Validate paths to prevent directory traversal
	if err := validateFilename(filename); err != nil {
		return nil, fmt.Errorf("invalid filename: %w", err)
	}

	// Security check - no directory traversal
	if strings.Contains(archivePath, "..") {
		return nil, fmt.Errorf("invalid archive path: contains directory traversal")
	}

	// Open the ZIP archive
	// The database stores relative paths like "lib/fb2-xxx.zip" from the working directory
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	// Find the file in the archive
	for _, f := range r.File {
		if f.Name == filename {
			file, err := f.Open()
			if err != nil {
				r.Close()
				return nil, fmt.Errorf("open file in archive: %w", err)
			}

			// Return a wrapper that closes both the file and the archive
			return &readCloser{
				ReadCloser: file,
				onClose: func() {
					r.Close()
				},
			}, nil
		}
	}

	r.Close()
	return nil, fmt.Errorf("file %s not found in archive", filename)
}

// ConvertFB2ToEPUB converts an FB2 file to EPUB format
// fb2Path is the path to the FB2 file (can be a temporary file)
// Returns a ReadCloser for the EPUB data
func (c *Converter) ConvertFB2ToEPUB(ctx context.Context, fb2Path string) (io.ReadCloser, string, error) {
	// Validate path
	if strings.Contains(fb2Path, "..") {
		return nil, "", fmt.Errorf("invalid FB2 path: contains directory traversal")
	}

	// Create a temporary directory for conversion
	tempDir, err := os.MkdirTemp("", "fb2convert-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}

	// Read the FB2 file to check encoding
	fb2Data, err := os.ReadFile(fb2Path)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("read FB2 file: %w", err)
	}

	// Check if the file has windows-1251 encoding declaration
	needsConversion := false
	xmlHeader := regexp.MustCompile(`<\?xml[^>]*encoding=["']([^"']+)["']`)
	if matches := xmlHeader.FindSubmatch(fb2Data); len(matches) > 1 {
		encoding := strings.ToLower(string(matches[1]))
		if encoding == "windows-1251" || encoding == "cp1251" {
			needsConversion = true
		}
	}

	var utf8FB2Path string
	if needsConversion {
		// Convert from windows-1251 to UTF-8
		decoder := charmap.Windows1251.NewDecoder()
		utf8Reader := transform.NewReader(bytes.NewReader(fb2Data), decoder)
		utf8Data, err := io.ReadAll(utf8Reader)
		if err != nil {
			os.RemoveAll(tempDir)
			return nil, "", fmt.Errorf("convert encoding: %w", err)
		}

		// Update XML declaration to UTF-8
		utf8Str := string(utf8Data)
		utf8Str = xmlHeader.ReplaceAllString(utf8Str, `<?xml version="1.0" encoding="UTF-8"?>`)

		// Write UTF-8 version to temp file
		utf8FB2Path = filepath.Join(tempDir, "book_utf8.fb2")
		if err := os.WriteFile(utf8FB2Path, []byte(utf8Str), 0644); err != nil {
			os.RemoveAll(tempDir)
			return nil, "", fmt.Errorf("write UTF-8 FB2: %w", err)
		}
	} else {
		utf8FB2Path = fb2Path
	}

	// Create output EPUB path
	epubPath := filepath.Join(tempDir, "converted.epub")

	// Create converter instance
	conv, err := converter.New(utf8FB2Path, 0)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("create FB2 converter: %w", err)
	}

	// Perform conversion (translit=false to keep Cyrillic characters)
	if err := conv.Convert(epubPath, false); err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("convert FB2 to EPUB: %w", err)
	}

	// Open the converted EPUB file
	epubFile, err := os.Open(epubPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("open converted EPUB: %w", err)
	}

	// Return a wrapper that cleans up when closed
	return &cleanupReadCloser{
		ReadCloser: epubFile,
		cleanup: func() {
			os.RemoveAll(tempDir)
		},
	}, epubPath, nil
}

// SanitizeFilename creates a safe filename from a book title
func SanitizeFilename(title string, format string) string {
	// Remove or replace characters that are unsafe for filenames
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	safe := title
	for _, ch := range unsafe {
		safe = strings.ReplaceAll(safe, ch, "_")
	}
	// Trim whitespace and limit length
	safe = strings.TrimSpace(safe)
	if len(safe) > 200 {
		safe = safe[:200]
	}
	return safe + "." + format
}

// FormatAuthorName formats an author's name as "LastName FirstName"
func FormatAuthorName(a book.Author) string {
	parts := []string{}
	if a.LastName != "" {
		parts = append(parts, a.LastName)
	}
	if a.FirstName != "" {
		parts = append(parts, a.FirstName)
	}
	return strings.Join(parts, " ")
}

// isUnknownAuthor checks if the author name indicates "unknown author"
func isUnknownAuthor(a book.Author) bool {
	unknownIndicators := []string{
		"неизвестный", "unknown", "автор неизвестен", "Автор неизвестен",
		"неизв.", "неизв", "anon", "anonymous",
	}

	lowerName := strings.ToLower(FormatAuthorName(a))
	for _, indicator := range unknownIndicators {
		if strings.Contains(lowerName, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

// FormatBookFilename creates a filename in "Author - Title.format" format
func FormatBookFilename(b *book.Book, format string) string {
	// Get author name(s)
	var authorName string
	var hasKnownAuthor bool

	if len(b.Author) > 0 {
		// Filter out unknown authors
		knownAuthors := []book.Author{}
		for _, a := range b.Author {
			if !isUnknownAuthor(a) {
				knownAuthors = append(knownAuthors, a)
			}
		}

		if len(knownAuthors) > 0 {
			hasKnownAuthor = true
			if len(knownAuthors) == 1 {
				authorName = FormatAuthorName(knownAuthors[0])
			} else {
				authorName = FormatAuthorName(knownAuthors[0]) + " et al."
			}
		}
	}

	// Sanitize title
	safeTitle := SanitizeFilename(b.Title, "")

	// Build filename
	var filename string
	if hasKnownAuthor && authorName != "" {
		safeAuthor := SanitizeFilename(authorName, "")
		filename = safeAuthor + " - " + safeTitle
	} else {
		// No known author, just use title
		filename = safeTitle
	}

	// Limit length
	if len(filename) > 200 {
		filename = filename[:200]
	}

	return filename + "." + format
}

// validatePath checks that a path is safe (no directory traversal)
func validatePath(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains directory traversal")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute")
	}
	return nil
}

// validateFilename checks that a filename is safe
func validateFilename(filename string) error {
	if strings.Contains(filename, "..") {
		return fmt.Errorf("filename contains directory traversal")
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("filename contains path separators")
	}
	return nil
}

// readCloser wraps a ReadCloser with an additional close callback
type readCloser struct {
	io.ReadCloser
	onClose func()
}

func (rc *readCloser) Close() error {
	var err1 error
	if rc.ReadCloser != nil {
		err1 = rc.ReadCloser.Close()
	}
	if rc.onClose != nil {
		rc.onClose()
	}
	return err1
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
