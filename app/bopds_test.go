package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestGetBooksHandler_Success(t *testing.T) {
	req := httptest.NewRequest("GET", "/b", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	svc := service.New(storage)
	handler := getBooksHandler(svc)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response is plain text (not JSON)
	contentType := w.Header().Get("Content-Type")
	if contentType != "" {
		t.Errorf("Expected no Content-Type for plain text, got %q", contentType)
	}

	// Check body is empty (no books in :memory: db)
	body := w.Body.String()
	if body != "" {
		t.Errorf("Expected empty body, got %q", body)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
