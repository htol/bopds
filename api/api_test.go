package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/service"
)

func init() {
	// Initialize logger for tests
	logger.Init("info")
}

func TestGetAuthorsByLetter_MissingParameter(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/authors", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := getAuthorsByLetterHandler(svc)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Check error message in JSON body
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if resp["error"] != "missing 'startsWith' query parameter" {
		t.Errorf("Expected error message 'missing 'startsWith' query parameter', got %q", resp["error"])
	}
}

func TestGetBooksByLetter_MissingParameter(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/books", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := getBooksByLetterHandler(svc)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Check error message in JSON body
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if resp["error"] != "missing 'startsWith' query parameter" {
		t.Errorf("Expected error message 'missing 'startsWith' query parameter', got %q", resp["error"])
	}
}

func TestRespondWithError(t *testing.T) {
	w := httptest.NewRecorder()

	testErr := &testError{msg: "test error"}
	respondWithError(w, "Test message", testErr, http.StatusBadGateway)

	if w.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Check error message in JSON body
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if resp["error"] != "Test message" {
		t.Errorf("Expected error message 'Test message', got %q", resp["error"])
	}
}

func TestRespondWithValidationError(t *testing.T) {
	w := httptest.NewRecorder()

	respondWithValidationError(w, "validation failed")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Check error message in JSON body
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if resp["error"] != "validation failed" {
		t.Errorf("Expected error message 'validation failed', got %q", resp["error"])
	}
}

func TestGetGenresHandler_Success(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/genres", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := getGenresHandler(svc)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Expected Content-Type to start with 'application/json', got %q", contentType)
	}

	// Check response is an array
	var resp []interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Empty genres array is expected (no data in :memory: db)
	if len(resp) != 0 {
		t.Errorf("Expected empty genres array, got %d items", len(resp))
	}
}

func TestHealthCheckHandler_Healthy(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := healthCheckHandler(svc)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Check health status
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %q", resp["status"])
	}
}

func TestGetBooksByLetterHandler_MissingParameter(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/books", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := getBooksByLetterHandler(svc)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Check error message in JSON body
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if resp["error"] != "missing 'startsWith' query parameter" {
		t.Errorf("Expected error message 'missing 'startsWith' query parameter', got %q", resp["error"])
	}
}

func TestGetBooksByLetterHandler_Success(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/books?startsWith=A", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := getBooksByLetterHandler(svc)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Check response is an array
	var resp []interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Empty books array is expected (no data in :memory: db)
	if len(resp) != 0 {
		t.Errorf("Expected empty books array, got %d items", len(resp))
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestIndexHandler_InjectsURLPrefix(t *testing.T) {
	dist := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dist, "frontend", "dist"), 0o755); err != nil {
		t.Fatalf("Failed to create dist directory: %v", err)
	}
	page := `<html><head><script>window.__URL_PREFIX__ = __BOPDS_URL_PREFIX__</script></head></html>`
	if err := os.WriteFile(filepath.Join(dist, "frontend", "dist", "index.html"), []byte(page), 0o644); err != nil {
		t.Fatalf("Failed to write test index.html: %v", err)
	}
	t.Chdir(dist)

	tests := []struct {
		name      string
		urlPrefix string
		expected  string
	}{
		{
			name:      "sub-path prefix",
			urlPrefix: "/lib",
			expected:  `window.__URL_PREFIX__ = "/lib"`,
		},
		{
			name:      "empty prefix",
			urlPrefix: "",
			expected:  `window.__URL_PREFIX__ = ""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			indexHandler(tt.urlPrefix).ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			if contentType := w.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
				t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got %q", contentType)
			}
			body := w.Body.String()
			if !strings.Contains(body, tt.expected) {
				t.Errorf("Expected body to contain %q, got %q", tt.expected, body)
			}
		})
	}
}

func TestBooksHandler_GroupsDuplicates(t *testing.T) {
	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	if _, err := storage.GetOrCreateLibrary("libA", "Library A"); err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}
	if _, err := storage.GetOrCreateLibrary("libB", ""); err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	author := []book.Author{{FirstName: "John", LastName: "Doe"}}
	add := func(library, filename string, libID, size int64) {
		b := &book.Book{
			Library:  library,
			Title:    "Duplicate Book",
			Author:   author,
			LibID:    libID,
			Archive:  "a.zip",
			FileName: filename,
			FileSize: size,
			Lang:     "en",
		}
		if err := storage.Add(b); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}
	add("libA", "1.fb2", 1, 100)
	add("libB", "2.fb2", 2, 200)

	svc := service.New(storage)
	handler := getBooksByLetterHandler(svc)
	req := httptest.NewRequest("GET", "/api/books?startsWith=D", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var groups []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&groups); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("Expected 1 grouped entry, got %d", len(groups))
	}

	copies, ok := groups[0]["copies"].([]interface{})
	if !ok {
		t.Fatalf("Expected 'copies' array, got %T", groups[0]["copies"])
	}
	if len(copies) != 2 {
		t.Fatalf("Expected 2 copies, got %d", len(copies))
	}

	wantLibraries := map[string]struct {
		displayName string
		fileSize    float64
	}{
		"libA": {displayName: "Library A", fileSize: 100},
		"libB": {displayName: "libB", fileSize: 200},
	}

	for _, c := range copies {
		copyMap, ok := c.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected copy object, got %T", c)
		}
		library, _ := copyMap["library"].(string)
		want, ok := wantLibraries[library]
		if !ok {
			t.Errorf("Unexpected library %q in copies", library)
			continue
		}
		if copyMap["library_display_name"] != want.displayName {
			t.Errorf("Library %s: display_name = %v, want %q", library, copyMap["library_display_name"], want.displayName)
		}
		if copyMap["file_size"] != want.fileSize {
			t.Errorf("Library %s: file_size = %v, want %v", library, copyMap["file_size"], want.fileSize)
		}
		bookID, _ := copyMap["book_id"].(float64)
		wantURL := fmt.Sprintf("/api/books/%d/download?format=fb2", int64(bookID))
		if copyMap["download_url"] != wantURL {
			t.Errorf("Library %s: download_url = %v, want %q", library, copyMap["download_url"], wantURL)
		}
	}
}

func TestOpdsFeed_LibraryName(t *testing.T) {
	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	if _, err := storage.GetOrCreateLibrary("libA", "Library A"); err != nil {
		t.Fatalf("GetOrCreateLibrary failed: %v", err)
	}

	b := &book.Book{
		Library:  "libA",
		Title:    "Sourced Book",
		Author:   []book.Author{{FirstName: "John", LastName: "Doe"}},
		LibID:    11,
		Archive:  "a.zip",
		FileName: "11.fb2",
		Lang:     "en",
	}
	if err := storage.Add(b); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	svc := service.New(storage)
	handler := opdsNewBooksHandler(svc)
	req := httptest.NewRequest("GET", "/opds/new", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<dc:source>Library A</dc:source>") {
		t.Errorf("Expected acquisition entry to carry library name in dc:source, got:\n%s", body)
	}
}
