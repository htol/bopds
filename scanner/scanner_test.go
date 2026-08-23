package scanner

import (
	"os"
	"path/filepath"
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

func TestScanLibraryEmptyDirNoHang(t *testing.T) {
	baseDir := t.TempDir()

	done := make(chan error)
	storage := Repo{}
	go func() {
		done <- ScanLibrary(baseDir, storage, 1000)
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

func TestScanLibraryInvalidInpxReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	storage := Repo{}
	os.WriteFile(filepath.Join(tmpDir, "bad.inpx"), []byte{127, 127}, 0o644)
	err := ScanLibrary(tmpDir, storage, 1000)
	if err == nil {
		t.Fatalf("didn't get error")
	}

}

func BenchmarkScanLibrary(b *testing.B) {
	// TODO: Update benchmark if needed
}
