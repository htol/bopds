package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/htol/bopds/api"
	"github.com/htol/bopds/book"
	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/service"
)

// fixtureRoots are the checked-in sample libraries (see testdata/libraries/README.md).
// The fixture must stay scannable and demonstrate both index styles, cross-library
// duplicates and display-name seeding; this test guards that.
func fixtureRoots(t *testing.T) []string {
	t.Helper()
	return []string{
		filepath.Join("..", "testdata", "libraries", "r1"),
		filepath.Join("..", "testdata", "libraries", "r2"),
	}
}

func TestFixture_LibrariesScan(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fixture.db")

	t.Setenv("DB_PATH", dbPath)
	t.Setenv("LIBRARY_PATH", strings.Join(fixtureRoots(t), ":"))
	t.Setenv("LIBRARY_NAMES", "libA:Fiction Library,libB:Tech Library")

	if code := CLI([]string{"scan"}); code != 0 {
		t.Fatalf("CLI scan of fixture exited with %d", code)
	}

	storage := repo.GetStorage(dbPath)
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	// Per-library book counts
	counts := make(map[string]int)
	var books []book.Book
	for _, letter := range []string{"A", "B", "G", "Q", "S", "T"} {
		result, err := storage.GetBooksByLetter(letter)
		if err != nil {
			t.Fatalf("GetBooksByLetter failed: %v", err)
		}
		books = append(books, result...)
	}
	for _, b := range books {
		counts[b.Library]++
	}
	if counts["libA"] != 6 {
		t.Errorf("Expected 6 books in libA, got %d", counts["libA"])
	}
	if counts["libB"] != 4 {
		t.Errorf("Expected 4 books in libB, got %d", counts["libB"])
	}

	// Display names seeded from LIBRARY_NAMES
	display := make(map[string]string)
	for _, b := range books {
		display[b.Library] = b.LibraryDisplayName
	}
	if display["libA"] != "Fiction Library" {
		t.Errorf("libA display name = %q, want %q", display["libA"], "Fiction Library")
	}
	if display["libB"] != "Tech Library" {
		t.Errorf("libB display name = %q, want %q", display["libB"], "Tech Library")
	}

	// Authors without a middle name (Last,First,) are parsed too
	authors, err := storage.GetAuthorsByLetter("K")
	if err != nil {
		t.Fatalf("GetAuthorsByLetter failed: %v", err)
	}
	if len(authors) != 1 || authors[0].LastName != "Kuznetsov" {
		t.Errorf("Expected author Kuznetsov, got %+v", authors)
	}

	// The UI defaults the language filter to "ru": demo books must be findable there
	searchResults, err := storage.SearchBooks(context.Background(), "Shadow", 10, 0, nil, []string{"ru"})
	if err != nil {
		t.Fatalf("SearchBooks failed: %v", err)
	}
	if len(searchResults) == 0 {
		t.Error("Expected 'Shadow' results under lang=ru (the UI default), got none")
	}

	// Cross-library duplicates group into one card with two copies
	svc := service.New(storage)
	groups, err := svc.GetBooksByLetterGrouped(context.Background(), "S")
	if err != nil {
		t.Fatalf("GetBooksByLetterGrouped failed: %v", err)
	}
	var shadowCopies int
	for _, g := range groups {
		if g.Title == "Shadow Protocol" {
			shadowCopies = len(g.Copies)
		}
	}
	if shadowCopies != 2 {
		t.Errorf("Expected 2 copies of the 'Shadow Protocol' duplicate, got %d", shadowCopies)
	}

	// Downloads resolve through the nested libB archive (.inpx source)
	libBID := int64(0)
	for _, b := range books {
		if b.Library == "libB" && b.Title == "Algorithms in Plain Words" {
			libBID = b.BookID
		}
	}
	if libBID == 0 {
		t.Fatal("libB book 'Algorithms in Plain Words' not found")
	}
	reader, _, _, err := svc.DownloadBookFB2(context.Background(), libBID)
	if err != nil {
		t.Fatalf("DownloadBookFB2 from libB failed: %v", err)
	}
	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !strings.Contains(string(content), "Algorithms in Plain Words") {
		t.Errorf("FB2 content mismatch, got: %s", string(content))
	}

	// The REST endpoint renders grouped duplicates for the UI
	handler := api.NewHandler(svc, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/books?startsWith=Q")
	if err != nil {
		t.Fatalf("GET /api/books failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Read response failed: %v", err)
	}
	if !strings.Contains(string(body), `"copies":[`) {
		t.Errorf("Expected grouped copies in REST response, got: %s", string(body))
	}
	if !strings.Contains(string(body), "Fiction Library") || !strings.Contains(string(body), "Tech Library") {
		t.Errorf("Expected library display names in REST response, got: %s", string(body))
	}
}
