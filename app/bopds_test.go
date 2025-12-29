package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/htol/bopds/repo"
)

func TestGetAuthorsByLetter_MissingParameter(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/authors", nil)
	w := httptest.NewRecorder()

	storage := repo.GetStorage(":memory:")
	defer func() {
		if err := storage.Close(); err != nil {
			t.Logf("Error closing storage: %v", err)
		}
	}()
	handler := getAuthorsByLetterHandler(storage)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	expectedBody := "missing 'startsWith' query parameter\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
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
	handler := getBooksByLetterHandler(storage)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	expectedBody := "missing 'startsWith' query parameter\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
	}
}

func TestRespondWithError(t *testing.T) {
	w := httptest.NewRecorder()

	testErr := &testError{msg: "test error"}
	respondWithError(w, "Test message", testErr, http.StatusBadGateway)

	if w.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", w.Code)
	}

	expectedBody := "Test message\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
