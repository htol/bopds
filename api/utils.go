package api

import (
	"encoding/json"
	"net/http"

	"github.com/htol/bopds/logger"
)

// respondWithError logs an error and sends an HTTP error response as JSON
func respondWithError(w http.ResponseWriter, message string, err error, statusCode int) {
	logger.Error(message, "error", err, "status", statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	}); err != nil {
		logger.Error("Failed to encode error response", "error", err)
	}
}

// respondWithValidationError sends a validation error response as JSON
func respondWithValidationError(w http.ResponseWriter, message string) {
	logger.Warn("Validation error", "message", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	}); err != nil {
		logger.Error("Failed to encode validation error", "error", err)
	}
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if r.Method == http.MethodOptions {
			return
		}
		h.ServeHTTP(w, r)
	})
}
