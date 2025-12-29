// Package app is the main cmd app
package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/scanner"
)

type Server struct {
	storage     *repo.Repo
	server      *http.Server
	portNumber  int
	libraryPath string
}

func NewServer(portNumber int, libraryPath string, storage *repo.Repo) *Server {
	return &Server{
		storage:     storage,
		portNumber:  portNumber,
		libraryPath: libraryPath,
	}
}

func (s *Server) Close() error {
	if s.storage != nil {
		s.storage.Close()
	}
	return nil
}

// respondWithError logs an error and sends an HTTP error response
func respondWithError(w http.ResponseWriter, message string, err error, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
}

func CLI(args []string) int {
	var app appEnv
	if err := app.fromArgs(args); err != nil {
		log.Println(err)
		return 2
	}

	if err := app.run(); err != nil {
		log.Printf("Runtime error: %v\n", err)
		return 1
	}
	return 0
}

type appEnv struct {
	server      *http.Server
	portNumber  int
	libraryPath string
	cmd         string
	storage     *repo.Repo
}

func (app *appEnv) fromArgs(args []string) error {
	fl := flag.NewFlagSet("bopds", flag.ContinueOnError)

	fl.IntVar(&app.portNumber, "p", 3001, "Port number (default 3001)")
	fl.StringVar(&app.libraryPath, "l", "./lib", "Path to library (default ./lib)")

	if err := fl.Parse(args); err != nil {
		fl.Usage()
		return err
	}

	if fl.NArg() < 1 {
		return fmt.Errorf("please provide a command to run")
	}

	app.cmd = fl.Arg(0)

	return nil
}

func (app *appEnv) run() error {
	storage := repo.GetStorage("books.db")
	defer storage.Close()

	switch app.cmd {
	case "scan":
		if err := scanner.ScanLibrary(app.libraryPath, storage); err != nil {
			return err
		}
	case "serve":
		app.storage = storage
		log.Printf("local access http://localhost:%d\n", app.portNumber)
		app.serve()
	case "init":
	default:
		return fmt.Errorf("unknown command %s", app.cmd)
	}
	return nil
}

func (app *appEnv) serve() {
	srv := &http.Server{Addr: fmt.Sprintf(":%d", app.portNumber),
		Handler: router(app.storage),
	}
	srv.ListenAndServe()
}

func router(storage *repo.Repo) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", indexHandler())
	mux.HandleFunc("/a", getAuthorsHandler(storage))
	mux.HandleFunc("/b", getBooksHandler(storage))
	mux.Handle("/api/authors", withCORS(getAuthorsByLetterHandler(storage)))
	mux.Handle("/api/books", withCORS(getBooksByLetterHandler(storage)))
	return mux
}

func indexHandler() http.Handler {
	return http.FileServer(http.Dir("./frontend/dist"))
}

func getAuthorsHandler(storage *repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authors, err := storage.GetAuthors()
		if err != nil {
			respondWithError(w, "Failed to get authors", err, http.StatusInternalServerError)
			return
		}
		for _, author := range authors {
			fmt.Fprintf(w, "%d: %s, %s, %s\n", storage.AuthorsCache[author], author.FirstName, author.MiddleName, author.LastName)
		}
	}
}

func getAuthorsByLetterHandler(storage *repo.Repo) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		letters := r.URL.Query().Get("startsWith")
		if letters == "" {
			http.Error(w, "missing 'startsWith' query parameter", http.StatusBadRequest)
			return
		}
		authors, err := storage.GetAuthorsByLetter(letters)
		if err != nil {
			respondWithError(w, "Failed to get authors by letter", err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(authors)
	}
	return http.HandlerFunc(hf)
}

func getBooksHandler(storage *repo.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		books, err := storage.GetBooks()
		if err != nil {
			respondWithError(w, "Failed to get books", err, http.StatusInternalServerError)
			return
		}

		for _, book := range books {
			fmt.Fprintf(w, "%s\n", book)
		}
	}
}

func getBooksByLetterHandler(storage *repo.Repo) http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		letters := r.URL.Query().Get("startsWith")
		if letters == "" {
			http.Error(w, "missing 'startsWith' query parameter", http.StatusBadRequest)
			return
		}
		books, err := storage.GetBooksByLetter(letters)
		if err != nil {
			respondWithError(w, "Failed to get books by letter", err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(books)
	}
	return http.HandlerFunc(hf)
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
