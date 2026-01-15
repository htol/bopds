package app

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/htol/bopds/logger"
	"github.com/htol/bopds/opds"
	"github.com/htol/bopds/service"
)

const (
	defaultPageSize    = 50
	maxPageSize        = 500
	opdsRootURL        = "/opds"
	opdsSearchURL      = "/opds/opensearch.xml"
	catalogTitle       = "bopds Library"
	catalogDescription = "OPDS Catalog for bopds eBook Library"
)

// respondWithOPDS writes an OPDS feed response with proper content type
func respondWithOPDS(w http.ResponseWriter, feed *opds.Feed, contentType string) {
	output, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		http.Error(w, "Failed to generate feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	w.Write(output)
}

// getBaseURL extracts the base URL from the request
func getBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Check for X-Forwarded-Proto header (common with reverse proxies)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// opdsRootHandler returns the OPDS catalog root (navigation feed)
func opdsRootHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := getBaseURL(r)

		feed := opds.NewNavigationFeed(
			"urn:uuid:bopds-root",
			catalogTitle,
			baseURL+opdsRootURL,
			baseURL+opdsRootURL,
		)

		// Add search link
		feed.AddSearchLink(baseURL + opdsSearchURL)

		// Add navigation entries
		feed.AddAcquisitionNavigationEntry(
			"urn:uuid:bopds-new",
			"New Books",
			baseURL+"/opds/new",
			opds.RelSortNew,
			"Recently added publications",
		)

		feed.AddNavigationEntry(
			"urn:uuid:bopds-authors",
			"Authors",
			baseURL+"/opds/authors",
			opds.RelSubsection,
			"Browse by author",
		)

		feed.AddNavigationEntry(
			"urn:uuid:bopds-genres",
			"Genres",
			baseURL+"/opds/genres",
			opds.RelSubsection,
			"Browse by genre",
		)

		respondWithOPDS(w, feed, opds.TypeNavigation)
	})
}

// opdsOpenSearchHandler returns the OpenSearch description XML
func opdsOpenSearchHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := getBaseURL(r)

		desc := opds.NewOpenSearchDescription(baseURL, catalogTitle, catalogDescription)
		output, err := desc.Marshal()
		if err != nil {
			http.Error(w, "Failed to generate OpenSearch description", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", opds.TypeOpenSearch+"; charset=utf-8")
		w.Write(output)
	})
}

// opdsSearchHandler returns search results as acquisition feed
func opdsSearchHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, "Missing search query parameter 'q'", http.StatusBadRequest)
			return
		}

		baseURL := getBaseURL(r)
		ctx := r.Context()

		// Parse pagination
		page := 1
		pageSize := defaultPageSize
		if p := r.URL.Query().Get("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				page = parsed
			}
		}
		if ps := r.URL.Query().Get("pageSize"); ps != "" {
			if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= maxPageSize {
				pageSize = parsed
			}
		}

		offset := (page - 1) * pageSize
		results, err := svc.SearchBooks(ctx, query, pageSize, offset)
		if err != nil {
			logger.Error("OPDS search failed", "query", query, "error", err)
			http.Error(w, "Search failed", http.StatusInternalServerError)
			return
		}

		feed := opds.NewAcquisitionFeed(
			fmt.Sprintf("urn:uuid:bopds-search-%s", query),
			fmt.Sprintf("Search: %s", query),
			fmt.Sprintf("%s/opds/search?q=%s", baseURL, query),
			baseURL+opdsRootURL,
		)
		feed.AddUpLink(baseURL+opdsRootURL, true)

		// Convert search results to book entries
		for _, result := range results {
			entry := opds.Entry{
				ID:       fmt.Sprintf("urn:uuid:bopds-book-%d", result.BookID),
				Title:    result.Title,
				Updated:  feed.Updated,
				Language: result.Lang,
				Links:    []opds.Link{},
			}

			// Add author
			if result.Author != "" {
				entry.Authors = append(entry.Authors, opds.Author{Name: result.Author})
			}

			// Add genres
			for _, genre := range result.Genres {
				entry.Categories = append(entry.Categories, opds.Category{Term: genre, Label: genre})
			}

			// Add acquisition links
			entry.Links = append(entry.Links,
				opds.Link{Rel: opds.RelAcquisitionOpen, Href: fmt.Sprintf("%s/api/books/%d/download?format=fb2.zip", baseURL, result.BookID), Type: "application/fb2+zip"},
				opds.Link{Rel: opds.RelAcquisitionOpen, Href: fmt.Sprintf("%s/api/books/%d/download?format=epub", baseURL, result.BookID), Type: "application/epub+zip"},
			)

			feed.Entries = append(feed.Entries, entry)
		}

		respondWithOPDS(w, feed, opds.TypeAcquisition)
	})
}

// opdsNewBooksHandler returns recently added books
func opdsNewBooksHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := getBaseURL(r)
		ctx := r.Context()

		// Parse pagination
		page := 1
		pageSize := defaultPageSize
		if p := r.URL.Query().Get("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				page = parsed
			}
		}
		if ps := r.URL.Query().Get("pageSize"); ps != "" {
			if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= maxPageSize {
				pageSize = parsed
			}
		}

		offset := (page - 1) * pageSize
		books, total, err := svc.GetRecentBooks(ctx, pageSize, offset)
		if err != nil {
			logger.Error("OPDS new books failed", "error", err)
			http.Error(w, "Failed to get new books", http.StatusInternalServerError)
			return
		}

		feed := opds.NewAcquisitionFeed(
			"urn:uuid:bopds-new",
			"New Books",
			fmt.Sprintf("%s/opds/new", baseURL),
			baseURL+opdsRootURL,
		)
		feed.AddUpLink(baseURL+opdsRootURL, true)

		for _, b := range books {
			feed.AddBookEntry(&b, baseURL)
		}

		// Add pagination
		feed.AddPaginationLinks(baseURL+"/opds/new", page, pageSize, total)

		respondWithOPDS(w, feed, opds.TypeAcquisition)
	})
}

// opdsAuthorsHandler returns author navigation feed
func opdsAuthorsHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := getBaseURL(r)
		ctx := r.Context()

		// Check for letter filter
		letter := r.URL.Query().Get("letter")

		feed := opds.NewNavigationFeed(
			"urn:uuid:bopds-authors",
			"Authors",
			baseURL+"/opds/authors",
			baseURL+opdsRootURL,
		)
		feed.AddUpLink(baseURL+opdsRootURL, true)

		if letter == "" {
			// Show alphabet navigation
			alphabet := "АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯABCDEFGHIJKLMNOPQRSTUVWXYZ"
			for _, char := range alphabet {
				letterStr := string(char)
				feed.AddNavigationEntry(
					fmt.Sprintf("urn:uuid:bopds-authors-%s", letterStr),
					letterStr,
					fmt.Sprintf("%s/opds/authors?letter=%s", baseURL, letterStr),
					opds.RelSubsection,
					fmt.Sprintf("Authors starting with %s", letterStr),
				)
			}
		} else {
			// Show authors for the selected letter
			authors, err := svc.GetAuthorsByLetter(ctx, letter)
			if err != nil {
				logger.Error("OPDS authors failed", "letter", letter, "error", err)
				http.Error(w, "Failed to get authors", http.StatusInternalServerError)
				return
			}

			feed.Title = fmt.Sprintf("Authors: %s", letter)
			feed.ID = fmt.Sprintf("urn:uuid:bopds-authors-%s", letter)

			for _, author := range authors {
				name := formatAuthorDisplayName(author.FirstName, author.MiddleName, author.LastName)
				feed.AddAcquisitionNavigationEntry(
					fmt.Sprintf("urn:uuid:bopds-author-%d", author.ID),
					name,
					fmt.Sprintf("%s/opds/authors/%d", baseURL, author.ID),
					opds.RelSubsection,
					fmt.Sprintf("%d books", author.BookCount),
				)
			}
		}

		respondWithOPDS(w, feed, opds.TypeNavigation)
	})
}

// opdsAuthorBooksHandler returns books by author (acquisition feed)
func opdsAuthorBooksHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid author ID", http.StatusBadRequest)
			return
		}

		baseURL := getBaseURL(r)
		ctx := r.Context()

		// Get author info
		author, err := svc.GetAuthorByID(ctx, id)
		if err != nil {
			logger.Error("OPDS author not found", "id", id, "error", err)
			http.Error(w, "Author not found", http.StatusNotFound)
			return
		}

		// Get author's books
		books, err := svc.GetBooksByAuthorID(ctx, id)
		if err != nil {
			logger.Error("OPDS author books failed", "id", id, "error", err)
			http.Error(w, "Failed to get author books", http.StatusInternalServerError)
			return
		}

		authorName := formatAuthorDisplayName(author.FirstName, author.MiddleName, author.LastName)

		feed := opds.NewAcquisitionFeed(
			fmt.Sprintf("urn:uuid:bopds-author-%d", id),
			authorName,
			fmt.Sprintf("%s/opds/authors/%d", baseURL, id),
			baseURL+opdsRootURL,
		)
		feed.AddUpLink(baseURL+"/opds/authors", true)

		for _, b := range books {
			feed.AddBookEntry(&b, baseURL)
		}

		respondWithOPDS(w, feed, opds.TypeAcquisition)
	})
}

// opdsGenresHandler returns genre navigation feed
func opdsGenresHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := getBaseURL(r)
		ctx := r.Context()

		genres, err := svc.GetGenres(ctx)
		if err != nil {
			logger.Error("OPDS genres failed", "error", err)
			http.Error(w, "Failed to get genres", http.StatusInternalServerError)
			return
		}

		feed := opds.NewNavigationFeed(
			"urn:uuid:bopds-genres",
			"Genres",
			baseURL+"/opds/genres",
			baseURL+opdsRootURL,
		)
		feed.AddUpLink(baseURL+opdsRootURL, true)

		for _, genre := range genres {
			feed.AddAcquisitionNavigationEntry(
				fmt.Sprintf("urn:uuid:bopds-genre-%s", genre),
				genre,
				fmt.Sprintf("%s/opds/genres/%s", baseURL, genre),
				opds.RelSubsection,
				"",
			)
		}

		respondWithOPDS(w, feed, opds.TypeNavigation)
	})
}

// opdsGenreBooksHandler returns books by genre (acquisition feed)
func opdsGenreBooksHandler(svc *service.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		genreName := r.PathValue("name")
		if genreName == "" {
			http.Error(w, "Missing genre name", http.StatusBadRequest)
			return
		}

		baseURL := getBaseURL(r)
		ctx := r.Context()

		// Parse pagination
		page := 1
		pageSize := defaultPageSize
		if p := r.URL.Query().Get("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				page = parsed
			}
		}
		if ps := r.URL.Query().Get("pageSize"); ps != "" {
			if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= maxPageSize {
				pageSize = parsed
			}
		}

		offset := (page - 1) * pageSize
		books, total, err := svc.GetBooksByGenre(ctx, genreName, pageSize, offset)
		if err != nil {
			logger.Error("OPDS genre books failed", "genre", genreName, "error", err)
			http.Error(w, "Failed to get genre books", http.StatusInternalServerError)
			return
		}

		feed := opds.NewAcquisitionFeed(
			fmt.Sprintf("urn:uuid:bopds-genre-%s", genreName),
			genreName,
			fmt.Sprintf("%s/opds/genres/%s", baseURL, genreName),
			baseURL+opdsRootURL,
		)
		feed.AddUpLink(baseURL+"/opds/genres", true)

		for _, b := range books {
			feed.AddBookEntry(&b, baseURL)
		}

		// Add pagination
		feed.AddPaginationLinks(fmt.Sprintf("%s/opds/genres/%s", baseURL, genreName), page, pageSize, total)

		respondWithOPDS(w, feed, opds.TypeAcquisition)
	})
}

// formatAuthorDisplayName formats author name for display
func formatAuthorDisplayName(firstName, middleName, lastName string) string {
	parts := []string{}
	if lastName != "" {
		parts = append(parts, lastName)
	}
	if firstName != "" {
		parts = append(parts, firstName)
	}
	if middleName != "" {
		parts = append(parts, middleName)
	}
	if len(parts) == 0 {
		return "Unknown Author"
	}
	return strings.Join(parts, " ")
}
