package api

import (
	"net/http"

	"github.com/htol/bopds/middleware"
	"github.com/htol/bopds/service"
)

// NewHandler creates and returns the main HTTP handler (router) for the application
func NewHandler(svc *service.Service, urlPrefix string) http.Handler {
	mux := http.NewServeMux()

	// OPDS Catalog routes
	mux.Handle("GET /opds", opdsRootHandler(svc))
	mux.Handle("GET /opds/", opdsRootHandler(svc))
	mux.Handle("GET /opds/opensearch.xml", opdsOpenSearchHandler(svc))
	mux.Handle("GET /opds/search", opdsSearchHandler(svc))
	mux.Handle("GET /opds/new", opdsNewBooksHandler(svc))
	mux.Handle("GET /opds/authors", opdsAuthorsHandler(svc))
	mux.Handle("GET /opds/authors/{id}", opdsAuthorBooksHandler(svc))
	mux.Handle("GET /opds/genres", opdsGenresHandler(svc))
	mux.Handle("GET /opds/genres/{name}", opdsGenreBooksHandler(svc))

	// Frontend and JSON API routes
	mux.Handle("GET /{$}", indexHandler(urlPrefix))
	mux.Handle("/", staticFilesHandler())
	mux.Handle("/api/authors", withCORS(getAuthorsByLetterHandler(svc)))
	mux.Handle("/api/authors/", withCORS(authorsAPIHandler(svc)))
	mux.Handle("/api/books", withCORS(getBooksByLetterHandler(svc)))
	mux.Handle("/api/books/", withCORS(downloadBookHandler(svc)))
	mux.Handle("/api/genres", withCORS(getGenresHandler(svc)))
	mux.Handle("/api/languages", withCORS(getLanguagesHandler(svc)))
	mux.Handle("/api/search", withCORS(searchBooksHandler(svc)))
	mux.HandleFunc("/health", healthCheckHandler(svc))

	// Apply URL prefix stripping if configured
	var handler http.Handler = mux
	if urlPrefix != "" {
		handler = http.StripPrefix(urlPrefix, mux)
	}

	// Apply middleware chain
	chain := middleware.Chain(
		middleware.Recovery,
		middleware.Logger,
		middleware.RequestID,
	)

	return chain(handler)
}
