// Package opds provides OPDS 1.2 catalog support
// OPDS (Open Publication Distribution System) is a syndication format for electronic
// publications based on Atom (RFC 4287).
package opds

import (
	"encoding/xml"
	"time"
)

// Namespaces
const (
	NamespaceAtom   = "http://www.w3.org/2005/Atom"
	NamespaceDC     = "http://purl.org/dc/terms/"
	NamespaceOpds   = "http://opds-spec.org/2010/catalog"
	NamespaceSearch = "http://a9.com/-/spec/opensearch/1.1/"
)

// Media Types
const (
	TypeNavigation  = "application/atom+xml;profile=opds-catalog;kind=navigation"
	TypeAcquisition = "application/atom+xml;profile=opds-catalog;kind=acquisition"
	TypeEntry       = "application/atom+xml;type=entry;profile=opds-catalog"
	TypeOpenSearch  = "application/opensearchdescription+xml"
)

// Acquisition Relations
const (
	RelAcquisition     = "http://opds-spec.org/acquisition"
	RelAcquisitionOpen = "http://opds-spec.org/acquisition/open-access"
	RelAcquisitionBuy  = "http://opds-spec.org/acquisition/buy"
)

// Image Relations
const (
	RelImage          = "http://opds-spec.org/image"
	RelImageThumbnail = "http://opds-spec.org/image/thumbnail"
)

// Sorting Relations
const (
	RelSortNew     = "http://opds-spec.org/sort/new"
	RelSortPopular = "http://opds-spec.org/sort/popular"
)

// Standard Link Relations (RFC 5988)
const (
	RelSelf       = "self"
	RelStart      = "start"
	RelUp         = "up"
	RelNext       = "next"
	RelPrevious   = "previous"
	RelFirst      = "first"
	RelLast       = "last"
	RelSubsection = "subsection"
	RelSearch     = "search"
	RelAlternate  = "alternate"
)

// Feed represents an OPDS Atom feed (navigation or acquisition)
type Feed struct {
	XMLName   xml.Name  `xml:"feed"`
	Xmlns     string    `xml:"xmlns,attr"`
	XmlnsDc   string    `xml:"xmlns:dc,attr,omitempty"`
	XmlnsOpds string    `xml:"xmlns:opds,attr,omitempty"`
	ID        string    `xml:"id"`
	Title     string    `xml:"title"`
	Updated   time.Time `xml:"updated"`
	Author    *Author   `xml:"author,omitempty"`
	Links     []Link    `xml:"link"`
	Entries   []Entry   `xml:"entry"`
}

// Entry represents an OPDS Atom entry (navigation item or book)
type Entry struct {
	ID         string     `xml:"id"`
	Title      string     `xml:"title"`
	Updated    time.Time  `xml:"updated"`
	Content    *Content   `xml:"content,omitempty"`
	Summary    string     `xml:"summary,omitempty"`
	Authors    []Author   `xml:"author,omitempty"`
	Links      []Link     `xml:"link"`
	Categories []Category `xml:"category,omitempty"`
	// Dublin Core extensions
	Language string `xml:"dc:language,omitempty"`
	Issued   string `xml:"dc:issued,omitempty"`
}

// Author represents an Atom author element
type Author struct {
	Name string `xml:"name"`
	URI  string `xml:"uri,omitempty"`
}

// Link represents an Atom link with OPDS extensions
type Link struct {
	Rel   string `xml:"rel,attr"`
	Href  string `xml:"href,attr"`
	Type  string `xml:"type,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
	// OPDS facet attributes (optional)
	FacetGroup  string `xml:"opds:facetGroup,attr,omitempty"`
	ActiveFacet string `xml:"opds:activeFacet,attr,omitempty"`
}

// Content represents Atom content element
type Content struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",chardata"`
}

// Category represents an Atom category (for genres)
type Category struct {
	Scheme string `xml:"scheme,attr,omitempty"`
	Term   string `xml:"term,attr"`
	Label  string `xml:"label,attr,omitempty"`
}
