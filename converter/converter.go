package converter

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
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
	if err := validateFilename(filename); err != nil {
		return nil, fmt.Errorf("invalid filename: %w", err)
	}

	if strings.Contains(archivePath, "..") {
		return nil, fmt.Errorf("invalid archive path: contains directory traversal")
	}

	// The database stores relative paths like "lib/fb2-xxx.zip" from the working directory
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	for _, f := range r.File {
		if f.Name == filename {
			file, err := f.Open()
			if err != nil {
				r.Close()
				return nil, fmt.Errorf("open file in archive: %w", err)
			}

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

// ExtractFrom7Z extracts an FB2 file from a 7z archive
func (c *Converter) ExtractFrom7Z(archivePath, filename string) (io.ReadCloser, error) {
	if err := validateFilename(filename); err != nil {
		return nil, fmt.Errorf("invalid filename: %w", err)
	}

	if strings.Contains(archivePath, "..") {
		return nil, fmt.Errorf("invalid archive path: contains directory traversal")
	}

	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open 7z archive: %w", err)
	}

	for _, f := range r.File {
		if f.Name == filename {
			file, err := f.Open()
			if err != nil {
				r.Close()
				return nil, fmt.Errorf("open file in archive: %w", err)
			}

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

// ExtractFromArchive extracts an FB2 file from a ZIP or 7z archive
// It auto-detects the archive type based on file extension
func (c *Converter) ExtractFromArchive(archivePath, filename string) (io.ReadCloser, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))

	switch ext {
	case ".zip":
		return c.ExtractFromZIP(archivePath, filename)
	case ".7z":
		return c.ExtractFrom7Z(archivePath, filename)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", ext)
	}
}

// ConvertFB2 converts an FB2 file to EPUB or MOBI format
func (c *Converter) ConvertFB2(ctx context.Context, fb2Path string, format string) (io.ReadCloser, string, error) {
	if strings.Contains(fb2Path, "..") {
		return nil, "", fmt.Errorf("invalid FB2 path: contains directory traversal")
	}

	if format != "epub" && format != "mobi" {
		return nil, "", fmt.Errorf("invalid format: must be 'epub' or 'mobi'")
	}

	tempDir, err := os.MkdirTemp("", "fb2convert-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}

	outputPath := filepath.Join(tempDir, "converted."+format)

	fb2Converter := fb2c.NewConverter()
	fb2Converter.SetOptions(fb2c.DefaultConvertOptions())

	if err := fb2Converter.Convert(fb2Path, outputPath); err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("convert FB2 to %s: %w", format, err)
	}

	convertedFile, err := os.Open(outputPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("open converted %s: %w", format, err)
	}

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
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	safe := title
	for _, ch := range unsafe {
		safe = strings.ReplaceAll(safe, ch, "_")
	}
	safe = strings.TrimRight(strings.TrimSpace(safe), ".")
	if len(safe) > 200 {
		safe = safe[:200]
	}
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
	var authorName string
	var hasKnownAuthor bool

	if len(b.Author) > 0 {
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

	safeTitle := SanitizeFilename(b.Title, "")

	var filename string
	if hasKnownAuthor && authorName != "" {
		safeAuthor := SanitizeFilename(authorName, "")
		filename = safeAuthor + " - " + safeTitle
	} else {
		filename = safeTitle
	}

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
