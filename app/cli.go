package app

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/htol/bopds/api"
	"github.com/htol/bopds/config"
	"github.com/htol/bopds/logger"
	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/scanner"
	"github.com/htol/bopds/service"
)

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
	server       *http.Server
	config       *config.Config
	libraryRoots []string
	scanName     string
	cmd          string
	storage      *repo.Repo
	service      *service.Service
	shutdownCtx  context.Context
}

// repeatableStrings collects repeatable flag values.
type repeatableStrings []string

func (r *repeatableStrings) String() string {
	return strings.Join(*r, ":")
}

func (r *repeatableStrings) Set(value string) error {
	for _, p := range strings.Split(value, ":") {
		if p != "" {
			*r = append(*r, p)
		}
	}
	return nil
}

func (app *appEnv) fromArgs(args []string) error {
	fl := flag.NewFlagSet("bopds", flag.ContinueOnError)

	// Load default config
	cfg := config.Load()

	// CLI flags override environment variables
	port := cfg.Server.Port
	var flagRoots repeatableStrings

	fl.IntVar(&port, "p", cfg.Server.Port, "Port number")
	fl.Var(&flagRoots, "l", "Path to library root (repeatable)")

	if err := fl.Parse(args); err != nil {
		fl.Usage()
		return err
	}

	if fl.NArg() < 1 {
		return fmt.Errorf("please provide a command to run")
	}

	app.cmd = fl.Arg(0)
	if fl.NArg() > 1 {
		app.scanName = fl.Arg(1)
	}
	app.libraryRoots = flagRoots
	if len(flagRoots) == 0 {
		app.libraryRoots = cfg.Library.Paths
	}
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
		// Prepare for fast scan
		if err := storage.InitCache(); err != nil {
			logger.Warn("Failed to initialize cache", "error", err)
		}

		if err := storage.SetFastMode(true); err != nil {
			logger.Warn("Failed to set fast mode", "error", err)
		}

		// Enable bulk import mode: 256MB cache, disabled WAL auto-checkpoint
		if err := storage.SetBulkImportMode(true); err != nil {
			logger.Warn("Failed to set bulk import mode", "error", err)
		}

		if err := storage.DropIndexes(); err != nil {
			logger.Warn("Failed to drop indexes (continuing anyway)", "error", err)
		} else {
			logger.Info("Indexes dropped for performance")
		}

		libraries, err := scanner.DiscoverLibraries(app.libraryRoots)
		if err != nil {
			return err
		}
		if app.scanName != "" {
			found := false
			for _, lib := range libraries {
				if lib.Name == app.scanName {
					libraries = []scanner.Library{lib}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("unknown library %q (use a name from LIBRARY_PATH roots: first-level subdirectories)", app.scanName)
			}
		}

		for _, lib := range libraries {
			libID, err := storage.GetOrCreateLibrary(lib.Name, app.config.Library.Names[lib.Name])
			if err != nil {
				return fmt.Errorf("register library %s: %w", lib.Name, err)
			}
			absRoot, err := filepath.Abs(lib.Root)
			if err != nil {
				return fmt.Errorf("resolve library root %s: %w", lib.Root, err)
			}
			if err := storage.SetLibraryPath(lib.Name, absRoot); err != nil {
				return fmt.Errorf("set library root %s: %w", lib.Name, err)
			}

			session := storage.BeginLibraryScan(libID)
			if err := scanner.ScanLibrary(lib, storage, app.config.Database.BatchSize); err != nil {
				return fmt.Errorf("scan library %s: %w", lib.Name, err)
			}
			if err := session.Finish(); err != nil {
				return fmt.Errorf("finish scan of library %s: %w", lib.Name, err)
			}
		}

		logger.Info("Recreating indexes...")
		if err := storage.CreateIndexes(); err != nil {
			return fmt.Errorf("recreate indexes: %w", err)
		}

		// Update genre display names and transliteration
		storage.SyncGenreDisplayNames()

		// Rebuild FTS index to populate author, series, and genre fields
		logger.Info("Rebuilding FTS index...")
		if err := storage.RebuildFTSIndex(); err != nil {
			return fmt.Errorf("rebuild FTS index: %w", err)
		}
		logger.Info("FTS index rebuilt successfully")

		// Restore normal mode
		if err := storage.SetFastMode(false); err != nil {
			logger.Warn("Failed to restore normal mode", "error", err)
		}

		// Disable bulk import mode and perform final checkpoint
		if err := storage.SetBulkImportMode(false); err != nil {
			logger.Warn("Failed to disable bulk import mode", "error", err)
		}

		// Checkpoint WAL to write all changes to disk
		if err := storage.CheckpointWAL(); err != nil {
			logger.Warn("Failed to checkpoint WAL", "error", err)
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
		Addr:    fmt.Sprintf(":%d", app.config.Server.Port),
		Handler: api.NewHandler(app.service, app.config.Server.URLPrefix),

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
