// Package app is the main cmd app
package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

// respondWithError logs an error and sends an HTTP error response
func respondWithError(w http.ResponseWriter, message string, err error, statusCode int) {
	logger.Error(message, "error", err, "status", statusCode)
	http.Error(w, message, statusCode)
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
		if err := scanner.ScanLibrary(app.libraryPath, storage); err != nil {
			return err
		}
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
	mux.Handle("/api/books", withCORS(getBooksByLetterHandler(svc)))
	mux.Handle("/api/genres", withCORS(getGenresHandler(svc)))
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
			http.Error(w, "missing 'startsWith' query parameter", http.StatusBadRequest)
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
			http.Error(w, "missing 'startsWith' query parameter", http.StatusBadRequest)
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
			logger.Error("Health check failed", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "unhealthy",
				"error":  "service unavailable",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
		})
	}
}
