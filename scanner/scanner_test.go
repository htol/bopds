package scanner

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
)

func init() {
	logger.Init("info")
}

func TestParseInpEntryWithAllFields(t *testing.T) {
	s := []string{
		"Author1,First,Middle:Author2,First,Middle:",
		"sf:fantasy:",
		"Test Book Title",
		"Great Series",       // flSeries
		"5",                  // flSerNo
		"12345",              // flFile
		"1024000",            // flSize
		"12345",              // flLibID
		"1",                  // flDeleted (book is present)
		"fb2",                // flExt
		"2024-01-15",         // flDate
		"ru",                 // flLang
		"5",                  // flLibRate
		"scifi space future", // flKeyWords
		"",                   // flURI (deprecated)
	}

	bookEntry := parseInpEntry(s)

	// Verify all fields
	if len(bookEntry.Author) != 2 {
		t.Errorf("Expected 2 authors, got %d", len(bookEntry.Author))
	}

	if bookEntry.Title != "Test Book Title" {
		t.Errorf("Expected 'Test Book Title', got '%s'", bookEntry.Title)
	}

	if bookEntry.Series == nil || bookEntry.Series.Name != "Great Series" {
		t.Errorf("Series not parsed correctly, got: %+v", bookEntry.Series)
	}

	if bookEntry.Series.SeriesNo != 5 {
		t.Errorf("Expected series no 5, got %d", bookEntry.Series.SeriesNo)
	}

	if bookEntry.FileName != "12345.fb2" {
		t.Errorf("Expected filename '12345.fb2', got '%s'", bookEntry.FileName)
	}

	if bookEntry.FileSize != 1024000 {
		t.Errorf("Expected file size 1024000, got %d", bookEntry.FileSize)
	}

	if bookEntry.LibID != 12345 {
		t.Errorf("Expected lib_id 12345, got %d", bookEntry.LibID)
	}

	if !bookEntry.Deleted {
		t.Errorf("Expected deleted=false (book is present), got true")
	}

	if bookEntry.DateAdded != "2024-01-15" {
		t.Errorf("Expected date '2024-01-15', got '%s'", bookEntry.DateAdded)
	}

	if bookEntry.LibRate != 5 {
		t.Errorf("Expected lib_rate 5, got %d", bookEntry.LibRate)
	}

	if len(bookEntry.Keywords) == 0 {
		t.Errorf("Keywords not parsed")
	}

	t.Logf("Parsed book: %+v", bookEntry)
}

func TestParseInpEntryEmptyOptionalFields(t *testing.T) {
	s := []string{
		"Author,First,Middle:",
		"sf:",
		"Simple Book",
		"", // flSeries (empty)
		"", // flSerNo (empty)
		"999",
		"50000",
		"999",
		"1",
		"fb2",
		"2024-01-01",
		"en",
		"", // flLibRate
		"", // flKeyWords
		"",
	}

	bookEntry := parseInpEntry(s)

	if bookEntry.Series != nil {
		t.Errorf("Expected no series, got %+v", bookEntry.Series)
	}

	if len(bookEntry.Keywords) != 0 {
		t.Errorf("Expected no keywords, got %d", len(bookEntry.Keywords))
	}

	if bookEntry.LibRate != 0 {
		t.Errorf("Expected lib_rate 0, got %d", bookEntry.LibRate)
	}

	t.Logf("Parsed book with empty optional fields: %+v", bookEntry)
}

func TestParseKeywords(t *testing.T) {
	// Test space-separated keywords
	kw := parseKeywords("scifi space future")
	if len(kw) != 3 {
		t.Errorf("Expected 3 keywords, got %d: %v", len(kw), kw)
	}

	// Test comma-separated keywords
	kw = parseKeywords("scifi,space,future")
	if len(kw) != 3 {
		t.Errorf("Expected 3 keywords, got %d: %v", len(kw), kw)
	}

	// Test empty keywords
	kw = parseKeywords("")
	if len(kw) != 0 {
		t.Errorf("Expected 0 keywords, got %d", len(kw))
	}

	// Test mixed spaces
	kw = parseKeywords("  scifi   space  ")
	if len(kw) != 2 || kw[0] != "scifi" || kw[1] != "space" {
		t.Errorf("Keywords not trimmed correctly: %v", kw)
	}
}

type Repo struct{}

func (r Repo) Add(*book.Book) error {
	return nil
}

func (r Repo) AddBatch([]*book.Book) error {
	return nil
}

// captureStorage records every book the scanner sends, for assertions.
type captureStorage struct {
	books []*book.Book
}

func (c *captureStorage) Add(b *book.Book) error {
	c.books = append(c.books, b)
	return nil
}

func (c *captureStorage) AddBatch(batch []*book.Book) error {
	// The scanner reuses the batch slice; the *book.Book pointers are distinct.
	c.books = append(c.books, batch...)
	return nil
}

// writeZip creates a valid empty zip archive at path.
func writeZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// inpLine joins .inp fields with the INPX field separator.
func inpLine(fields ...string) string {
	return strings.Join(fields, string(rune(4)))
}

func TestScanLibraries_LooseInp(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "foo-lib")
	if err := os.Mkdir(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(libDir, "foo.inp"), []byte(inpLine(
		"Ivanov,Ivan,Ivanovich:", // authors: Last,First,Middle:
		"sf:fantasy:",             // genres, trailing list separator
		"Test Book",
		"", // series
		"", // series no
		"12345",
		"1024000",
		"777",
		"0", // deleted: 0 = present
		"fb2",
		"2024-01-15",
		"ru",
		"", // lib rate
		"", // keywords
		"", // uri
	)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(libDir, "foo.zip"))

	storage := &captureStorage{}
	if err := ScanLibraries([]string{root}, storage, 1000); err != nil {
		t.Fatal(err)
	}

	if len(storage.books) != 1 {
		t.Fatalf("Expected 1 book, got %d: %+v", len(storage.books), storage.books)
	}
	b := storage.books[0]

	if b.Library != "foo-lib" {
		t.Errorf("Library = %q, want %q", b.Library, "foo-lib")
	}
	if b.Archive != "foo.zip" {
		t.Errorf("Archive = %q, want %q (relative to library root)", b.Archive, "foo.zip")
	}
	if b.Title != "Test Book" {
		t.Errorf("Title = %q, want %q", b.Title, "Test Book")
	}
	if b.FileName != "12345.fb2" {
		t.Errorf("FileName = %q, want %q", b.FileName, "12345.fb2")
	}
	if b.FileSize != 1024000 {
		t.Errorf("FileSize = %d, want 1024000", b.FileSize)
	}
	if b.LibID != 777 {
		t.Errorf("LibID = %d, want 777", b.LibID)
	}
	if b.Lang != "ru" {
		t.Errorf("Lang = %q, want %q", b.Lang, "ru")
	}
	if b.DateAdded != "2024-01-15" {
		t.Errorf("DateAdded = %q, want %q", b.DateAdded, "2024-01-15")
	}
	if b.Deleted {
		t.Error("Deleted = true, want false")
	}
	if len(b.Author) != 1 || b.Author[0].LastName != "Ivanov" || b.Author[0].FirstName != "Ivan" || b.Author[0].MiddleName != "Ivanovich" {
		t.Errorf("Author = %+v, want one Ivanov/Ivan/Ivanovich", b.Author)
	}
	if len(b.Genres) != 2 || b.Genres[0] != "sf" || b.Genres[1] != "fantasy" {
		t.Errorf("Genres = %v, want [sf fantasy]", b.Genres)
	}
}

// writeInpx creates an .inpx archive at path containing one .inp entry with the given lines.
func writeInpx(t *testing.T, path, inpName string, lines ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	entry, err := w.Create(inpName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(strings.Join(lines, "\n") + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestScanLibraries_InpxNestedArchive(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	subDir := filepath.Join(libDir, "sub")
	for _, dir := range []string{libDir, subDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeInpx(t, filepath.Join(libDir, "X.inpx"), "X.inp", inpLine("", "", "Nested Book"))
	writeZip(t, filepath.Join(subDir, "X.zip"))

	storage := &captureStorage{}
	if err := ScanLibraries([]string{root}, storage, 1000); err != nil {
		t.Fatal(err)
	}

	if len(storage.books) != 1 {
		t.Fatalf("Expected 1 book, got %d: %+v", len(storage.books), storage.books)
	}
	b := storage.books[0]
	if b.Title != "Nested Book" {
		t.Errorf("Title = %q, want %q", b.Title, "Nested Book")
	}
	if b.Archive != filepath.Join("sub", "X.zip") {
		t.Errorf("Archive = %q, want %q", b.Archive, filepath.Join("sub", "X.zip"))
	}
	if b.Library != "lib" {
		t.Errorf("Library = %q, want %q", b.Library, "lib")
	}
}

func TestScanLibraries_InpxPrefersAdjacentArchiveOverSiblingZip(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	coversDir := filepath.Join(libDir, "covers")
	for _, dir := range []string{libDir, coversDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Index at the library root. A same-named zip of cover images sits in
	// covers/; the real book archive (.7z) lies next to the index. The
	// scanner never opens companions during scan, so a stub .7z is enough.
	writeInpx(t, filepath.Join(libDir, "X.inpx"), "X.inp", inpLine("", "", "Real Book"))
	writeZip(t, filepath.Join(coversDir, "X.zip"))
	if err := os.WriteFile(filepath.Join(libDir, "X.7z"), []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	storage := &captureStorage{}
	if err := ScanLibraries([]string{root}, storage, 1000); err != nil {
		t.Fatal(err)
	}

	if len(storage.books) != 1 {
		t.Fatalf("Expected 1 book, got %d: %+v", len(storage.books), storage.books)
	}
	b := storage.books[0]
	if b.Archive != "X.7z" {
		t.Errorf("Archive = %q, want the adjacent X.7z, not covers/X.zip", b.Archive)
	}
}

func TestScanLibraries_MixedSources(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Loose .inp with companion at the library root
	if err := os.WriteFile(filepath.Join(libDir, "loose.inp"), []byte(inpLine("", "", "Loose Book")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(libDir, "loose.zip"))

	// .inpx with its own, disjoint archive in a subdirectory
	writeInpx(t, filepath.Join(libDir, "idx.inpx"), "idx.inp", inpLine("", "", "Indexed Book"))
	nested := filepath.Join(libDir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(nested, "idx.zip"))

	storage := &captureStorage{}
	if err := ScanLibraries([]string{root}, storage, 1000); err != nil {
		t.Fatal(err)
	}

	if len(storage.books) != 2 {
		t.Fatalf("Expected 2 books, got %d: %+v", len(storage.books), storage.books)
	}

	byTitle := map[string]*book.Book{}
	for _, b := range storage.books {
		byTitle[b.Title] = b
	}

	loose, ok := byTitle["Loose Book"]
	if !ok {
		t.Fatalf("Loose Book not imported: %+v", storage.books)
	}
	if loose.Library != "lib" || loose.Archive != "loose.zip" {
		t.Errorf("Loose Book: Library = %q, Archive = %q; want lib, loose.zip", loose.Library, loose.Archive)
	}

	indexed, ok := byTitle["Indexed Book"]
	if !ok {
		t.Fatalf("Indexed Book not imported: %+v", storage.books)
	}
	if indexed.Library != "lib" || indexed.Archive != filepath.Join("nested", "idx.zip") {
		t.Errorf("Indexed Book: Library = %q, Archive = %q; want lib, %s", indexed.Library, indexed.Archive, filepath.Join("nested", "idx.zip"))
	}
}

func TestScanLibrariesEmptyDirNoHang(t *testing.T) {
	baseDir := t.TempDir()

	done := make(chan error)
	storage := Repo{}
	go func() {
		done <- ScanLibraries([]string{baseDir}, storage, 1000)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("error returned")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("didn't return")
	}
}

func TestScanLibrariesInvalidInpxReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "lib")
	if err := os.Mkdir(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	storage := Repo{}
	if err := os.WriteFile(filepath.Join(libDir, "bad.inpx"), []byte{127, 127}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ScanLibraries([]string{tmpDir}, storage, 1000); err == nil {
		t.Fatalf("didn't get error")
	}
}

func BenchmarkScanLibrary(b *testing.B) {
	// TODO: Update benchmark if needed
}
