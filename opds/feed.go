package opds

import (
	"fmt"
	"time"

	"github.com/htol/bopds/book"
)

// NewNavigationFeed creates a new navigation feed
func NewNavigationFeed(id, title, selfURL, startURL string) *Feed {
	return &Feed{
		Xmlns:     NamespaceAtom,
		XmlnsOpds: NamespaceOpds,
		XmlnsDc:   NamespaceDC, // Optional but good for compatibility
		ID:        id,
		Title:     title,
		Updated:   time.Now().UTC(),
		Author: &Author{
			Name: "bopds",
			URI:  "https://github.com/htol/bopds",
		},
		Links: []Link{
			{Rel: RelSelf, Href: selfURL, Type: TypeNavigation},
			{Rel: RelStart, Href: startURL, Type: TypeNavigation},
		},
		Entries: []Entry{},
	}
}

// NewAcquisitionFeed creates a new acquisition feed
func NewAcquisitionFeed(id, title, selfURL, startURL string) *Feed {
	return &Feed{
		Xmlns:     NamespaceAtom,
		XmlnsDc:   NamespaceDC,
		XmlnsOpds: NamespaceOpds,
		ID:        id,
		Title:     title,
		Updated:   time.Now().UTC(),
		Author: &Author{
			Name: "bopds",
			URI:  "https://github.com/htol/bopds",
		},
		Links: []Link{
			{Rel: RelSelf, Href: selfURL, Type: TypeAcquisition},
			{Rel: RelStart, Href: startURL, Type: TypeNavigation},
		},
		Entries: []Entry{},
	}
}

// AddSearchLink adds an OpenSearch link to the feed
func (f *Feed) AddSearchLink(searchURL string) {
	f.Links = append(f.Links, Link{
		Rel:  RelSearch,
		Href: searchURL,
		Type: TypeOpenSearch,
	})
}

// AddUpLink adds a parent navigation link
func (f *Feed) AddUpLink(upURL string, isNavigation bool) {
	linkType := TypeAcquisition
	if isNavigation {
		linkType = TypeNavigation
	}
	f.Links = append(f.Links, Link{
		Rel:  RelUp,
		Href: upURL,
		Type: linkType,
	})
}

// AddNavigationEntry adds a navigation entry to the feed
func (f *Feed) AddNavigationEntry(id, title, href, rel, content string) {
	entry := Entry{
		ID:      id,
		Title:   title,
		Updated: time.Now().UTC(),
		Links: []Link{
			{Rel: rel, Href: href, Type: TypeNavigation},
		},
	}
	if content != "" {
		entry.Content = &Content{Type: "text", Value: content}
	}
	f.Entries = append(f.Entries, entry)
}

// AddAcquisitionNavigationEntry adds a navigation entry that links to an acquisition feed
func (f *Feed) AddAcquisitionNavigationEntry(id, title, href, rel, content string) {
	entry := Entry{
		ID:      id,
		Title:   title,
		Updated: time.Now().UTC(),
		Links: []Link{
			{Rel: rel, Href: href, Type: TypeAcquisition},
		},
	}
	if content != "" {
		entry.Content = &Content{Type: "text", Value: content}
	}
	f.Entries = append(f.Entries, entry)
}

// AddBookEntry adds a book entry with acquisition links
func (f *Feed) AddBookEntry(b *book.Book, baseURL string) {
	entry := Entry{
		ID:      fmt.Sprintf("urn:uuid:bopds-book-%d", b.BookID),
		Title:   b.Title,
		Updated: time.Now().UTC(),
		Links:   []Link{},
	}

	// Add authors
	for _, author := range b.Author {
		name := formatAuthorName(author)
		entry.Authors = append(entry.Authors, Author{Name: name})
	}

	// Add language
	if b.Lang != "" {
		entry.Language = b.Lang
	}

	// Add genres as categories
	for _, genre := range b.Genres {
		entry.Categories = append(entry.Categories, Category{
			Term:  genre,
			Label: genre,
		})
	}

	// Add acquisition links for different formats
	// FB2 zipped (primary format)
	entry.Links = append(entry.Links, Link{
		Rel:  RelAcquisitionOpen,
		Href: fmt.Sprintf("%s/api/books/%d/download?format=fb2.zip", baseURL, b.BookID),
		Type: "application/fb2+zip",
	})

	// EPUB
	entry.Links = append(entry.Links, Link{
		Rel:  RelAcquisitionOpen,
		Href: fmt.Sprintf("%s/api/books/%d/download?format=epub", baseURL, b.BookID),
		Type: "application/epub+zip",
	})

	// MOBI
	entry.Links = append(entry.Links, Link{
		Rel:  RelAcquisitionOpen,
		Href: fmt.Sprintf("%s/api/books/%d/download?format=mobi", baseURL, b.BookID),
		Type: "application/x-mobipocket-ebook",
	})

	f.Entries = append(f.Entries, entry)
}

// AddPaginationLinks adds next/prev links for RFC 5005 pagination
func (f *Feed) AddPaginationLinks(baseURL string, page, pageSize, total int) {
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// First page
	if page > 1 {
		f.Links = append(f.Links, Link{
			Rel:  RelFirst,
			Href: fmt.Sprintf("%s?page=1&pageSize=%d", baseURL, pageSize),
			Type: TypeAcquisition,
		})
	}

	// Previous page
	if page > 1 {
		f.Links = append(f.Links, Link{
			Rel:  RelPrevious,
			Href: fmt.Sprintf("%s?page=%d&pageSize=%d", baseURL, page-1, pageSize),
			Type: TypeAcquisition,
		})
	}

	// Next page
	if page < totalPages {
		f.Links = append(f.Links, Link{
			Rel:  RelNext,
			Href: fmt.Sprintf("%s?page=%d&pageSize=%d", baseURL, page+1, pageSize),
			Type: TypeAcquisition,
		})
	}

	// Last page
	if page < totalPages {
		f.Links = append(f.Links, Link{
			Rel:  RelLast,
			Href: fmt.Sprintf("%s?page=%d&pageSize=%d", baseURL, totalPages, pageSize),
			Type: TypeAcquisition,
		})
	}
}

// formatAuthorName formats an author's full name
func formatAuthorName(a book.Author) string {
	name := ""
	if a.FirstName != "" {
		name = a.FirstName
	}
	if a.MiddleName != "" {
		if name != "" {
			name += " "
		}
		name += a.MiddleName
	}
	if a.LastName != "" {
		if name != "" {
			name += " "
		}
		name += a.LastName
	}
	if name == "" {
		name = "Unknown Author"
	}
	return name
}
