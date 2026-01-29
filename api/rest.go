package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/htol/bopds/logger"
	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/service"
)

func indexHandler() http.Handler {
	return http.FileServer(http.Dir("./frontend/dist"))
}

func getAuthorsByLetterHandler(svc *service.Service) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		letters := r.URL.Query().Get("startsWith")
		if letters == "" {
			respondWithValidationError(w, "missing 'startsWith' query parameter")
			return
		}
		ctx := r.Context()
		authors, err := svc.GetAuthorsByLetter(ctx, letters)
		if err != nil {
			respondWithError(w, "Failed to get authors by letter", err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(authors); err != nil {
			logger.Error("Failed to encode authors response", "error", err)
		}
	}
	return http.HandlerFunc(hf)
}

func getAuthorByIDHandler(svc *service.Service) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		// Extract author ID from URL: /api/authors/123
		path := strings.TrimPrefix(r.URL.Path, "/api/authors/")
		path = strings.TrimSuffix(path, "/books")

		id, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			respondWithValidationError(w, "invalid author ID")
			return
		}

		ctx := r.Context()
		author, err := svc.GetAuthorByID(ctx, id)
		if err != nil {
			if err == repo.ErrNotFound {
				respondWithError(w, "author not found", err, http.StatusNotFound)
			} else {
				respondWithError(w, "Failed to get author", err, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(author); err != nil {
			logger.Error("Failed to encode author response", "error", err)
		}
	}
	return http.HandlerFunc(hf)
}

func getBooksByAuthorIDHandler(svc *service.Service) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		// Extract author ID from URL: /api/authors/123/books
		path := strings.TrimPrefix(r.URL.Path, "/api/authors/")
		path = strings.TrimSuffix(path, "/books")

		id, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			respondWithValidationError(w, "invalid author ID")
			return
		}

		ctx := r.Context()
		books, err := svc.GetBooksByAuthorID(ctx, id)
		if err != nil {
			respondWithError(w, "Failed to get books by author", err, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(books); err != nil {
			logger.Error("Failed to encode books response", "error", err)
		}
	}
	return http.HandlerFunc(hf)
}

func searchBooksHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Parse and validate query parameter
		query := r.URL.Query().Get("q")
		if query == "" {
			respondWithValidationError(w, "missing 'q' query parameter")
			return
		}

		// Parse limit with validation
		limitStr := r.URL.Query().Get("limit")
		limit := 20 // default
		if limitStr != "" {
			l, err := strconv.Atoi(limitStr)
			if err != nil {
				respondWithValidationError(w, "invalid 'limit' parameter")
				return
			}
			if l < 1 || l > 100 {
				respondWithValidationError(w, "'limit' must be between 1 and 100")
				return
			}
			limit = l
		}

		// Parse offset with validation
		offsetStr := r.URL.Query().Get("offset")
		offset := 0 // default
		if offsetStr != "" {
			o, err := strconv.Atoi(offsetStr)
			if err != nil {
				respondWithValidationError(w, "invalid 'offset' parameter")
				return
			}
			if o < 0 {
				respondWithValidationError(w, "'offset' must be >= 0")
				return
			}
			offset = o
		}

		// Parse fields
		fieldsStr := r.URL.Query().Get("fields")
		var fields []string
		if fieldsStr != "" {
			fields = strings.Split(fieldsStr, ",")
			// Validate fields
			for _, f := range fields {
				switch f {
				case "title", "author", "genre", "series":
					// valid
				default:
					respondWithValidationError(w, fmt.Sprintf("invalid field '%s'", f))
					return
				}
			}
		}

		// Parse languages
		langsStr := r.URL.Query().Get("lang")
		var languages []string
		if langsStr != "" {
			languages = strings.Split(langsStr, ",")
		}

		// Perform search with context for cancellation
		results, err := svc.SearchBooks(ctx, query, limit, offset, fields, languages)
		if err != nil {
			respondWithError(w, "Failed to search books", err, http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(results); err != nil {
			logger.Error("Failed to encode search results", "error", err)
		}
	})
}
func authorsAPIHandler(svc *service.Service) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		// Check if the path ends with /books
		if strings.HasSuffix(r.URL.Path, "/books") {
			getBooksByAuthorIDHandler(svc).ServeHTTP(w, r)
		} else {
			// Otherwise, treat it as get author by ID
			getAuthorByIDHandler(svc).ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(hf)
}

func getBooksByLetterHandler(svc *service.Service) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		letters := r.URL.Query().Get("startsWith")
		if letters == "" {
			respondWithValidationError(w, "missing 'startsWith' query parameter")
			return
		}
		ctx := r.Context()
		books, err := svc.GetBooksByLetter(ctx, letters)
		if err != nil {
			respondWithError(w, "Failed to get books by letter", err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(books); err != nil {
			logger.Error("Failed to encode books response", "error", err)
		}
	}
	return http.HandlerFunc(hf)
}

func getGenresHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		genres, err := svc.GetGenres(ctx)
		if err != nil {
			respondWithError(w, "Failed to get genres", err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(genres); err != nil {
			logger.Error("Failed to encode genres response", "error", err)
		}
	})
}

// getLanguagesHandler handles the languages list endpoint
func getLanguagesHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		languages, err := svc.GetLanguages(ctx)
		if err != nil {
			respondWithError(w, "Failed to fetch languages", err, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(languages); err != nil {
			logger.Error("Failed to encode response", "error", err)
		}
	})
}

func healthCheckHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check service health (database connection via service layer)
		if err := svc.Ping(ctx); err != nil {
			respondWithError(w, "service unavailable", err, http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
		}); err != nil {
			logger.Error("Failed to encode health check response", "error", err)
		}
	}
}

func downloadBookHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract book ID from URL: /api/books/123/download?format=fb2
		path := strings.TrimPrefix(r.URL.Path, "/api/books/")
		path = strings.TrimSuffix(path, "/download")

		// Parse book ID
		id, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			respondWithValidationError(w, "invalid book ID")
			return
		}

		// Get format parameter
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "fb2" // default
		}

		if format != "fb2" && format != "fb2.zip" && format != "epub" && format != "mobi" {
			respondWithValidationError(w, "format must be 'fb2', 'fb2.zip', 'epub' or 'mobi'")
			return
		}

		var reader io.ReadCloser
		var filename string
		var size int64

		switch format {
		case "fb2":
			reader, filename, size, err = svc.DownloadBookFB2(ctx, id)
		case "fb2.zip":
			reader, filename, size, err = svc.DownloadBookFB2Zip(ctx, id)
		case "epub":
			reader, filename, size, err = svc.DownloadBookEPUB(ctx, id)
		case "mobi":
			reader, filename, size, err = svc.DownloadBookMOBI(ctx, id)
		}

		if err != nil {
			if err == repo.ErrNotFound {
				respondWithError(w, "book not found", err, http.StatusNotFound)
			} else {
				respondWithError(w, "failed to prepare download", err, http.StatusInternalServerError)
			}
			return
		}
		defer reader.Close()

		if size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		}

		// Set headers for file download
		switch format {
		case "fb2":
			w.Header().Set("Content-Type", "application/fb2+xml")
		case "fb2.zip":
			w.Header().Set("Content-Type", "application/zip")
		case "epub":
			w.Header().Set("Content-Type", "application/epub+zip")
		case "mobi":
			w.Header().Set("Content-Type", "application/x-mobipocket-ebook")
		}

		// Set filename with proper UTF-8 encoding (RFC 5987)
		encodedFilename := url.PathEscape(filename)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFilename))

		// Stream file to response
		_, err = io.Copy(w, reader)
		if err != nil {
			logger.Error("failed to stream file", "error", err, "book_id", id)
		}
	})
}
