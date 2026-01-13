// Package app is the main cmd app
package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/htol/bopds/config"
	"github.com/htol/bopds/logger"
	"github.com/htol/bopds/middleware"
	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/scanner"
	"github.com/htol/bopds/service"
)

type Server struct {
	storage     *repo.Repo
	service     *service.Service
	server      *http.Server
	config      *config.Config
	libraryPath string
}

func NewServer(libraryPath string, storage *repo.Repo, cfg *config.Config) *Server {
	return &Server{
		storage:     storage,
		service:     service.New(storage),
		config:      cfg,
		libraryPath: libraryPath,
	}
}

func (s *Server) Close() error {
	if s.storage != nil {
		if err := s.storage.Close(); err != nil {
			return err
		}
	}
	return nil
}

// respondWithError logs an error and sends an HTTP error response as JSON
func respondWithError(w http.ResponseWriter, message string, err error, statusCode int) {
	logger.Error(message, "error", err, "status", statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}

// respondWithValidationError sends a validation error response as JSON
func respondWithValidationError(w http.ResponseWriter, message string) {
	logger.Warn("Validation error", "message", message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}

func CLI(args []string) int {
	var app appEnv
	if err := app.fromArgs(args); err != nil {
		fmt.Println(err)
		return 2
	}

	if err := app.run(); err != nil {
		logger.Error("Runtime error", "error", err)
		return 1
	}
	return 0
}

type appEnv struct {
	server      *http.Server
	config      *config.Config
	libraryPath string
	cmd         string
	storage     *repo.Repo
	service     *service.Service
	shutdownCtx context.Context
}

func (app *appEnv) fromArgs(args []string) error {
	fl := flag.NewFlagSet("bopds", flag.ContinueOnError)

	// Load default config
	cfg := config.Load()

	// CLI flags override environment variables
	port := cfg.Server.Port
	libPath := cfg.Library.Path

	fl.IntVar(&port, "p", cfg.Server.Port, "Port number")
	fl.StringVar(&libPath, "l", cfg.Library.Path, "Path to library")

	if err := fl.Parse(args); err != nil {
		fl.Usage()
		return err
	}

	if fl.NArg() < 1 {
		return fmt.Errorf("please provide a command to run")
	}

	app.cmd = fl.Arg(0)
	app.libraryPath = libPath
	app.config = cfg
	app.config.Server.Port = port

	return nil
}

func (app *appEnv) run() error {
	// Initialize logger
	logger.Init(app.config.LogLevel)

	storage := repo.GetStorage(app.config.Database.Path)

	switch app.cmd {
	case "scan":
		defer func() {
			if err := storage.Close(); err != nil {
				logger.Error("Error closing storage", "error", err)
			}
		}()
		if err := storage.DropIndexes(); err != nil {
			logger.Warn("Failed to drop indexes (continuing anyway)", "error", err)
		} else {
			logger.Info("Indexes dropped for performance")
		}

		if err := scanner.ScanLibrary(app.libraryPath, storage, app.config.Database.BatchSize); err != nil {
			return err
		}

		logger.Info("Recreating indexes...")
		if err := storage.CreateIndexes(); err != nil {
			return fmt.Errorf("recreate indexes: %w", err)
		}
		// Rebuild FTS index to populate author, series, and genre fields
		logger.Info("Rebuilding FTS index...")
		if err := storage.RebuildFTSIndex(); err != nil {
			return fmt.Errorf("rebuild FTS index: %w", err)
		}
		logger.Info("FTS index rebuilt successfully")
	case "serve":
		app.storage = storage
		app.service = service.New(storage)
		app.serve()
	case "init":
		defer func() {
			if err := storage.Close(); err != nil {
				logger.Error("Error closing storage", "error", err)
			}
		}()
	case "rebuild":
		defer func() {
			if err := storage.Close(); err != nil {
				logger.Error("Error closing storage", "error", err)
			}
		}()
		if err := storage.RebuildFTSIndex(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown command %s", app.cmd)
	}
	return nil
}

func (app *appEnv) serve() {
	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	app.shutdownCtx = shutdownCtx

	// Create server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.Server.Port),
		Handler:      router(app.service),
		ReadTimeout:  time.Duration(app.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(app.config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(app.config.Server.IdleTimeout) * time.Second,
	}
	app.server = srv

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Server listening", "port", app.config.Server.Port, "url", fmt.Sprintf("http://localhost:%d", app.config.Server.Port))
		serverErrors <- srv.ListenAndServe()
	}()

	// Wait for interrupt signal
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal or server errors
	select {
	case err := <-serverErrors:
		// Server failed to start
		if err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
		}
		return
	case sig := <-shutdownSignal:
		// Received shutdown signal
		logger.Info("Received shutdown signal", "signal", sig.String())

		// Initiate graceful shutdown
		logger.Info("Shutting down server...")
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("Server shutdown error", "error", err)
		}

		// Close database connection
		logger.Info("Closing database connection...")
		if err := app.storage.Close(); err != nil {
			logger.Error("Error closing storage", "error", err)
		}

		logger.Info("Server stopped")
	}
}

func router(svc *service.Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", indexHandler())
	mux.HandleFunc("/a", getAuthorsHandler(svc))
	mux.HandleFunc("/b", getBooksHandler(svc))
	mux.Handle("/api/authors", withCORS(getAuthorsByLetterHandler(svc)))
	mux.Handle("/api/authors/", withCORS(authorsAPIHandler(svc)))
	mux.Handle("/api/books", withCORS(getBooksByLetterHandler(svc)))
	mux.Handle("/api/books/", withCORS(downloadBookHandler(svc)))
	mux.Handle("/api/genres", withCORS(getGenresHandler(svc)))
	mux.Handle("/api/search", withCORS(searchBooksHandler(svc)))
	mux.HandleFunc("/health", healthCheckHandler(svc))

	// Apply middleware chain
	chain := middleware.Chain(
		middleware.Recovery,
		middleware.Logger,
		middleware.RequestID,
	)

	return chain(mux)
}

func indexHandler() http.Handler {
	return http.FileServer(http.Dir("./frontend/dist"))
}

func getAuthorsHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		authors, err := svc.GetAuthors(ctx)
		if err != nil {
			respondWithError(w, "Failed to get authors", err, http.StatusInternalServerError)
			return
		}
		for _, author := range authors {
			fmt.Fprintf(w, "%s, %s, %s\n", author.FirstName, author.MiddleName, author.LastName)
		}
	}
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
		json.NewEncoder(w).Encode(authors)
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
		json.NewEncoder(w).Encode(author)
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
		json.NewEncoder(w).Encode(books)
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

		// Perform search with context for cancellation
		results, err := svc.SearchBooks(ctx, query, limit, offset)
		if err != nil {
			respondWithError(w, "Failed to search books", err, http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
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

func getBooksHandler(svc *service.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		books, err := svc.GetBooks(ctx)
		if err != nil {
			respondWithError(w, "Failed to get books", err, http.StatusInternalServerError)
			return
		}

		for _, book := range books {
			fmt.Fprintf(w, "%s\n", book)
		}
	}
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
		json.NewEncoder(w).Encode(books)
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
		json.NewEncoder(w).Encode(genres)
	})
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
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
		})
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

		switch format {
		case "fb2":
			reader, filename, err = svc.DownloadBookFB2(ctx, id)
		case "fb2.zip":
			reader, filename, err = svc.DownloadBookFB2Zip(ctx, id)
		case "epub":
			reader, filename, err = svc.DownloadBookEPUB(ctx, id)
		case "mobi":
			reader, filename, err = svc.DownloadBookMOBI(ctx, id)
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
		// Use percent-encoding for UTF-8 filename
		encodedFilename := url.PathEscape(filename)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFilename))

		// Stream file to response
		_, err = io.Copy(w, reader)
		if err != nil {
			logger.Error("failed to stream file", "error", err, "book_id", id)
		}
	})
}
