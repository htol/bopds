package app

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/htol/bopds/repo"
	"github.com/htol/bopds/scanner"
)

var storage *repo.Repo

func CLI(args []string) int {
	var app appEnv
	if err := app.fromArgs(args); err != nil {
		fmt.Println(err)
		return 2
	}

	if err := app.run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		return 1
	}
	return 0
}

type appEnv struct {
	server      *http.Server
	portNumber  int
	libraryPath string
	cmd         string
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
	switch app.cmd {
	case "scan":
		if err := scanner.ScanLibrary(app.libraryPath); err != nil {
			return err
		}
	case "serve":
		storage = repo.GetStorage("books.db")
		defer storage.Close()
		log.Printf("local access http://localhost:%d\n", app.portNumber)
		app.serve()
	case "init":
		storage = repo.GetStorage("books.db")
		defer storage.Close()
	default:
		return fmt.Errorf("unknown command %s", app.cmd)
	}
	return nil
}

func (app *appEnv) serve() {
	srv := &http.Server{Addr: fmt.Sprintf(":%d", app.portNumber),
		Handler: router(),
	}
	srv.ListenAndServe()
}

func router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/a", getAuthors)
	mux.HandleFunc("/b", getBooks)
	return mux
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<h1>Hello!<h1>"))
}

func getAuthors(w http.ResponseWriter, r *http.Request) {
	authors, err := storage.GetAuthors()
	if err != nil {
		log.Fatal("getAuthor: ", err)
	}
	for _, author := range authors {
		fmt.Fprintf(w, "%d: %s, %s, %s\n", storage.AuthorsCache[author], author.FirstName, author.MiddleName, author.LastName)
	}
}

func getBooks(w http.ResponseWriter, r *http.Request) {
	books, err := storage.GetBooks()
	if err != nil {
		log.Fatal("getBooks: ", err)
	}

	for _, book := range books {
		fmt.Fprintf(w, "%s\n", book)
	}
}
