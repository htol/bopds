package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverLibraries(t *testing.T) {
	t.Run("first-level subdirectories across roots", func(t *testing.T) {
		r1 := t.TempDir()
		r2 := t.TempDir()

		libA := filepath.Join(r1, "libA")
		libB := filepath.Join(r2, "libB") // left empty: a library is a subdirectory, content optional
		for _, dir := range []string{libA, libB} {
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(r1, "notes.txt"), []byte("not a library"), 0o644); err != nil {
			t.Fatal(err)
		}

		libs, err := DiscoverLibraries([]string{r1, r2})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(libs) != 2 {
			t.Fatalf("Expected 2 libraries, got %d: %+v", len(libs), libs)
		}
		want := []Library{
			{Name: "libA", Root: r1, Path: libA},
			{Name: "libB", Root: r2, Path: libB},
		}
		for i, w := range want {
			if libs[i] != w {
				t.Errorf("libs[%d] = %+v, want %+v", i, libs[i], w)
			}
		}
	})

	t.Run("duplicate name across roots", func(t *testing.T) {
		r1 := t.TempDir()
		r2 := t.TempDir()
		dup1 := filepath.Join(r1, "dup")
		dup2 := filepath.Join(r2, "dup")
		for _, dir := range []string{dup1, dup2} {
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}

		_, err := DiscoverLibraries([]string{r1, r2})
		if err == nil {
			t.Fatal("Expected an error for duplicate library names, got none")
		}
		for _, path := range []string{dup1, dup2} {
			if !strings.Contains(err.Error(), path) {
				t.Errorf("Expected error to name %s, got: %v", path, err)
			}
		}
	})
}
