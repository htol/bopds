package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/htol/bopds/repo"
)

func init() {
	os.Setenv("LOG_LEVEL", "error")
}

// scanFixture builds two roots, each with one library containing one book.
func scanFixture(t *testing.T) (root1, root2 string) {
	t.Helper()

	makeLibrary := func(root, lib, name, title string) string {
		libDir := filepath.Join(root, lib)
		if err := os.MkdirAll(libDir, 0o755); err != nil {
			t.Fatal(err)
		}

		line := strings.Join([]string{
			"", "", title, "", "", name, "100", "1", "0", "fb2", "2024-01-01", "en", "", "", "",
		}, string(rune(4)))

		if err := os.WriteFile(filepath.Join(libDir, name+".inp"), []byte(line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(libDir, name+".zip"))
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		if _, err := zw.Create(name + ".fb2"); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		f.Close()
		return libDir
	}

	root1 = t.TempDir()
	root2 = t.TempDir()
	makeLibrary(root1, "libA", "a", "Alpha Book")
	makeLibrary(root2, "libB", "b", "Beta Book")
	return root1, root2
}

// booksByLibrary returns the number of visible books per library name.
func booksByLibrary(t *testing.T, dbPath string) map[string]int {
	t.Helper()

	storage := repo.GetStorage(dbPath)
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	result := make(map[string]int)
	for _, letter := range []string{"A", "B"} {
		books, err := storage.GetBooksByLetter(letter)
		if err != nil {
			t.Fatalf("GetBooksByLetter failed: %v", err)
		}
		for _, b := range books {
			result[b.Library]++
		}
	}
	return result
}

func TestCLI_ScanSingleLibrary(t *testing.T) {
	root1, root2 := scanFixture(t)
	dbPath := filepath.Join(t.TempDir(), "cli.db")

	t.Setenv("DB_PATH", dbPath)
	t.Setenv("LIBRARY_PATH", strings.Join([]string{root1, root2}, ":"))

	// scan libA imports only libA
	if code := CLI([]string{"scan", "libA"}); code != 0 {
		t.Fatalf("CLI scan libA exited with %d", code)
	}
	counts := booksByLibrary(t, dbPath)
	if counts["libA"] != 1 {
		t.Errorf("Expected 1 book in libA, got %d", counts["libA"])
	}
	if counts["libB"] != 0 {
		t.Errorf("Expected 0 books in libB after scanning only libA, got %d", counts["libB"])
	}

	// full scan imports all
	if code := CLI([]string{"scan"}); code != 0 {
		t.Fatalf("CLI scan exited with %d", code)
	}
	counts = booksByLibrary(t, dbPath)
	if counts["libA"] != 1 || counts["libB"] != 1 {
		t.Errorf("Expected 1 book per library after full scan, got %v", counts)
	}
}

func TestCLI_ScanRepeatableLibraryFlag(t *testing.T) {
	root1, root2 := scanFixture(t)
	dbPath := filepath.Join(t.TempDir(), "cli.db")

	t.Setenv("DB_PATH", dbPath)

	if code := CLI([]string{"-l", root1, "-l", root2, "scan"}); code != 0 {
		t.Fatalf("CLI scan exited with %d", code)
	}

	counts := booksByLibrary(t, dbPath)
	if counts["libA"] != 1 || counts["libB"] != 1 {
		t.Errorf("Expected 1 book per library with repeatable -l, got %v", counts)
	}

	// Rescan is stable: same natural keys, no duplicates
	if code := CLI([]string{"-l", root1, "-l", root2, "scan"}); code != 0 {
		t.Fatalf("CLI rescan exited with %d", code)
	}
	counts = booksByLibrary(t, dbPath)
	if counts["libA"] != 1 || counts["libB"] != 1 {
		t.Errorf("Expected stable book counts after rescan, got %v", counts)
	}
}

func TestCLI_ScanUnknownLibraryName(t *testing.T) {
	root1, _ := scanFixture(t)
	dbPath := filepath.Join(t.TempDir(), "cli.db")

	t.Setenv("DB_PATH", dbPath)
	t.Setenv("LIBRARY_PATH", root1)

	code := CLI([]string{"scan", "nosuchlib"})
	if code == 0 {
		t.Error("Expected non-zero exit code for unknown library name")
	}
}
