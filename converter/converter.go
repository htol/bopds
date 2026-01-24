package converter

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"

	"github.com/bodgit/sevenzip"
	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
	"github.com/htol/fb2c"
)

// Converter handles FB2 extraction and EPUB conversions
type Converter struct{}

// New creates a new Converter
func New() *Converter {
	return &Converter{}
}

// ExtractFromZIP extracts an FB2 file from a ZIP archive
func (c *Converter) ExtractFromZIP(archivePath, filename string) (io.ReadCloser, int64, error) {
	if err := validateFilename(filename); err != nil {
		return nil, 0, fmt.Errorf("invalid filename: %w", err)
	}

	if strings.Contains(archivePath, "..") {
		return nil, 0, fmt.Errorf("invalid archive path: contains directory traversal")
	}

	// The database stores relative paths like "lib/fb2-xxx.zip" from the working directory
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, 0, fmt.Errorf("open zip archive: %w", err)
	}

	for _, f := range r.File {
		if f.Name == filename {
			file, err := f.Open()
			if err != nil {
				r.Close()
				return nil, 0, fmt.Errorf("open file in archive: %w", err)
			}

			return &readCloser{
				ReadCloser: file,
				onClose: func() {
					r.Close()
				},
			}, int64(f.UncompressedSize64), nil
		}
	}

	r.Close()
	return nil, 0, fmt.Errorf("file %s not found in archive", filename)
}

// ExtractFrom7Z extracts an FB2 file from a 7z archive
func (c *Converter) ExtractFrom7Z(archivePath, filename string) (io.ReadCloser, int64, error) {
	if err := validateFilename(filename); err != nil {
		return nil, 0, fmt.Errorf("invalid filename: %w", err)
	}

	start := time.Now()

	// Try native Go extraction first (supports LZMA, LZMA2, Deflate, etc.)
	rc, size, err := c.extractFrom7zNative(archivePath, filename)
	if err == nil {
		logger.Info("7z extraction completed (native)", "archive", archivePath, "file", filename, "duration", time.Since(start).Milliseconds())
		return rc, size, nil
	}

	// Fallback for algorithms unsupported by pure Go lib (e.g. PPMd, BCJ)
	if strings.Contains(err.Error(), "unsupported compression algorithm") {
		startCLI := time.Now()
		rc, size, err := c.extractWith7zCLI(archivePath, filename)
		if err == nil {
			logger.Info("7z extraction completed (CLI fallback)", "archive", archivePath, "file", filename, "duration", time.Since(startCLI).Milliseconds())
			return rc, size, nil
		}
	}

	return nil, 0, err
}

func (c *Converter) extractFrom7zNative(archivePath, filename string) (io.ReadCloser, int64, error) {
	if strings.Contains(archivePath, "..") {
		return nil, 0, fmt.Errorf("invalid archive path: contains directory traversal")
	}

	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, 0, fmt.Errorf("open 7z archive: %w", err)
	}

	for _, f := range r.File {
		if f.Name == filename {
			file, err := f.Open()
			if err != nil {
				r.Close()
				return nil, 0, fmt.Errorf("open file in archive: %w", err)
			}

			return &readCloser{
				ReadCloser: file,
				onClose: func() {
					r.Close()
				},
			}, int64(f.UncompressedSize), nil
		}
	}

	r.Close()
	return nil, 0, fmt.Errorf("file %s not found in archive", filename)
}

func (c *Converter) extractWith7zCLI(archivePath, filename string) (io.ReadCloser, int64, error) {
	// Use system 7z binary to extract to stdout
	cmd := exec.Command("7z", "e", "-so", archivePath, filename)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, -1, fmt.Errorf("7z cli pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, -1, fmt.Errorf("7z cli start: %w", err)
	}

	return &cmdReadCloser{
		ReadCloser: stdout,
		cmd:        cmd,
	}, -1, nil // Size unknown for stream extraction
}

type cmdReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReadCloser) Close() error {
	readErr := c.ReadCloser.Close()
	// Wait for process to finish (it should exit after pipe close or completion)
	// We ignore exit error if it's just SIGPIPE due to early close
	_ = c.cmd.Wait()
	return readErr
}

// ExtractFromArchive extracts an FB2 file from a ZIP or 7z archive
// It auto-detects the archive type based on file extension
func (c *Converter) ExtractFromArchive(archivePath, filename string) (io.ReadCloser, int64, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))

	switch ext {
	case ".zip":
		return c.ExtractFromZIP(archivePath, filename)
	case ".7z":
		return c.ExtractFrom7Z(archivePath, filename)
	default:
		return nil, 0, fmt.Errorf("unsupported archive format: %s", ext)
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

	start := time.Now()
	fb2Converter := fb2c.NewConverter()
	fb2Converter.SetOptions(fb2c.DefaultConvertOptions())

	if err := fb2Converter.Convert(fb2Path, outputPath); err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("convert FB2 to %s: %w", format, err)
	}

	convertDuration := time.Since(start).Milliseconds()
	logger.Info("FB2 conversion completed", "format", format, "path", fb2Path, "duration", convertDuration)

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
