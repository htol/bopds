package converter

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/htol/bopds/book"
	"github.com/htol/fb2c"
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

// ConvertFB2 converts an FB2 file to the specified format (epub or mobi)
// fb2Path is the path to the FB2 file (can be a temporary file)
// format should be "epub" or "mobi"
// Returns a ReadCloser for the converted data and the output path
func (c *Converter) ConvertFB2(ctx context.Context, fb2Path string, format string) (io.ReadCloser, string, error) {
	// Validate path
	if strings.Contains(fb2Path, "..") {
		return nil, "", fmt.Errorf("invalid FB2 path: contains directory traversal")
	}

	// Validate format
	if format != "epub" && format != "mobi" {
		return nil, "", fmt.Errorf("invalid format: must be 'epub' or 'mobi'")
	}

	// Create a temporary directory for conversion
	tempDir, err := os.MkdirTemp("", "fb2convert-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}

	// Create output path with appropriate extension
	outputPath := filepath.Join(tempDir, "converted."+format)

	// Use fb2c library for conversion (it handles encoding conversion internally)
	fb2Converter := fb2c.NewConverter()
	fb2Converter.SetOptions(fb2c.DefaultConvertOptions())

	// Perform conversion
	if err := fb2Converter.Convert(fb2Path, outputPath); err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("convert FB2 to %s: %w", format, err)
	}

	// Open the converted file
	convertedFile, err := os.Open(outputPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("open converted %s: %w", format, err)
	}

	// Return a wrapper that cleans up when closed
	return &cleanupReadCloser{
		ReadCloser: convertedFile,
		cleanup: func() {
			os.RemoveAll(tempDir)
		},
	}, outputPath, nil
}

// ConvertFB2ToEPUB converts an FB2 file to EPUB format
// Maintained for backward compatibility
func (c *Converter) ConvertFB2ToEPUB(ctx context.Context, fb2Path string) (io.ReadCloser, string, error) {
	return c.ConvertFB2(ctx, fb2Path, "epub")
}

// ConvertFB2ToMOBI converts an FB2 file to MOBI format
func (c *Converter) ConvertFB2ToMOBI(ctx context.Context, fb2Path string) (io.ReadCloser, string, error) {
	return c.ConvertFB2(ctx, fb2Path, "mobi")
}

// SanitizeFilename creates a safe filename from a book title
func SanitizeFilename(title string, format string) string {
	// Remove or replace characters that are unsafe for filenames
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	safe := title
	for _, ch := range unsafe {
		safe = strings.ReplaceAll(safe, ch, "_")
	}
	// Trim whitespace and trailing dots
	safe = strings.TrimRight(strings.TrimSpace(safe), ".")
	// Limit length
	if len(safe) > 200 {
		safe = safe[:200]
	}
	// Add format extension if provided
	if format != "" {
		return safe + "." + format
	}
	return safe
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
