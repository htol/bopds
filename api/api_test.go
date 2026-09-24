package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

func TestGetBooksBySeriesHandler(t *testing.T) {
	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()

	for _, lib := range []struct{ name, display string }{
		{"libA", "Library A"},
		{"libB", ""},
	} {
		if _, err := storage.GetOrCreateLibrary(lib.name, lib.display); err != nil {
			t.Fatalf("GetOrCreateLibrary failed: %v", err)
		}
	}

	var seriesID int64
	resolveSeriesID := func() {
		t.Helper()
		if seriesID != 0 {
			return
		}
		all, err := storage.GetSeries()
		if err != nil {
			t.Fatalf("GetSeries failed: %v", err)
		}
		for _, s := range all {
			if s.Name == "API Saga" {
				seriesID = s.ID
				return
			}
		}
		t.Fatal("Series 'API Saga' not found")
	}
	add := func(library, title, filename string, seriesNo int) {
		t.Helper()
		b := &book.Book{
			Library:  library,
			Title:    title,
			Author:   []book.Author{{FirstName: "John", LastName: "Doe"}},
			Lang:     "en",
			Archive:  "books.zip",
			FileName: filename,
			Series:   &book.SeriesInfo{ID: seriesID, Name: "API Saga", SeriesNo: seriesNo},
		}
		if err := storage.Add(b); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		resolveSeriesID()
	}
	// Insert out of order; duplicates of "Alpha" in two libraries merge into one entry
	add("libA", "Beta Volume", "2.fb2", 2)
	add("libA", "Alpha Volume", "1a.fb2", 1)
	add("libB", "Alpha Volume", "1b.fb2", 1)

	handler := NewHandler(service.New(storage), "")

	t.Run("success grouped and ordered", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/series/%d/books", seriesID), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var groups []map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&groups); err != nil {
			t.Fatalf("Failed to decode JSON response: %v", err)
		}
		if len(groups) != 2 {
			t.Fatalf("Expected 2 grouped entries, got %d", len(groups))
		}
		if groups[0]["title"] != "Alpha Volume" || groups[1]["title"] != "Beta Volume" {
			t.Errorf("Expected series-number order, got [%v, %v]", groups[0]["title"], groups[1]["title"])
		}

		copies, ok := groups[0]["copies"].([]interface{})
		if !ok || len(copies) != 2 {
			t.Fatalf("Expected 2 copies for the cross-library duplicate, got %v", groups[0]["copies"])
		}
		libraries := map[string]bool{}
		for _, c := range copies {
			copyMap, _ := c.(map[string]interface{})
			libraries[copyMap["library"].(string)] = true
		}
		if !libraries["libA"] || !libraries["libB"] {
			t.Errorf("Expected copies in libA and libB, got %v", libraries)
		}

		series, _ := groups[0]["series"].(map[string]interface{})
		if series == nil || series["name"] != "API Saga" || int64(series["series_id"].(float64)) != seriesID {
			t.Errorf("Expected series info with ID %d, got %v", seriesID, series)
		}

		authors, _ := groups[0]["authors"].([]interface{})
		if len(authors) != 1 {
			t.Fatalf("Expected 1 author, got %v", authors)
		}
		author, _ := authors[0].(map[string]interface{})
		if id, _ := author["ID"].(float64); id == 0 {
			t.Errorf("Expected non-zero author ID in grouped response, got %v", author)
		}
	})

	t.Run("malformed ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/series/abc/books", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("unknown series", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/series/999999/books", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
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

// capturingHandler records every log entry the handler emits
type capturingHandler struct {
	records []slog.Record
}

func (h *capturingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *capturingHandler) WithGroup(_ string) slog.Handler { return h }

func TestSearchBooksHandler_ClientCanceled(t *testing.T) {
	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := searchBooksHandler(svc)

	// The client disconnects before the search starts
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/api/search?q=test", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	caps := &capturingHandler{}
	prevLogger := logger.Logger
	logger.Logger = slog.New(caps)
	defer func() { logger.Logger = prevLogger }()

	handler.ServeHTTP(w, req)

	// No 500 response attempt: nothing was written
	if w.Code == http.StatusInternalServerError {
		t.Errorf("Expected no 500 for a canceled request, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("Expected empty body for a canceled request, got %q", w.Body.String())
	}

	// Cancellation is informational, never an ERROR
	canceledAtInfo := false
	for _, rec := range caps.records {
		if rec.Level == slog.LevelError {
			t.Errorf("Expected no ERROR-level log for a canceled request, got %q", rec.Message)
		}
		if rec.Message == "client canceled search" && rec.Level == slog.LevelInfo {
			canceledAtInfo = true
		}
	}
	if !canceledAtInfo {
		t.Errorf("Expected an Info-level 'client canceled search' entry, got %v", caps.records)
	}
}

func TestSearchBooksHandler_DatabaseError(t *testing.T) {
	storage := repo.GetStorage(":memory:")
	if err := storage.Close(); err != nil {
		t.Fatalf("Failed to close storage: %v", err)
	}
	svc := service.New(storage)
	handler := searchBooksHandler(svc)

	req := httptest.NewRequest("GET", "/api/search?q=test", nil)
	w := httptest.NewRecorder()

	caps := &capturingHandler{}
	prevLogger := logger.Logger
	logger.Logger = slog.New(caps)
	defer func() { logger.Logger = prevLogger }()

	handler.ServeHTTP(w, req)

	// A genuine failure still surfaces as ERROR + 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for a database error, got %d", w.Code)
	}
	loggedError := false
	for _, rec := range caps.records {
		if rec.Level == slog.LevelError {
			loggedError = true
		}
	}
	if !loggedError {
		t.Errorf("Expected an ERROR-level log entry for a database error")
	}
}
