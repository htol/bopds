package book

import "encoding/xml"

type Author struct {
	XMLName    xml.Name `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 author" json:"-"`
	ID         int64    `xml:"-"` // No json tag - won't be in JSON by default
	FirstName  string   `xml:"first-name"`
	MiddleName string   `xml:"middle-name"`
	LastName   string   `xml:"last-name"`
}

// AuthorWithID includes ID in JSON (for API responses)
type AuthorWithID struct {
	Author
	ID int64 `json:"ID"`
}

// AuthorWithBookCount represents an author with their book count
type AuthorWithBookCount struct {
	ID         int64    `json:"ID"`
	FirstName  string   `json:"FirstName"`
	MiddleName string   `json:"MiddleName"`
	LastName   string   `json:"LastName"`
	BookCount  int      `json:"BookCount"`
}

type Book struct {
	BookID   int64    `xml:"-" json:"book_id,omitempty"`
	XMLName  xml.Name `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 title-info"`
	Author   []Author `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 author"`
	Title    string   `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 book-title"`
	Lang     string   `xml:"lang"`
	Genres   []string `xml:""`
	Archive  string
	FileName string

	// ===== NEW FIELDS FROM INPX =====
	FileSize  int64       `json:"file_size,omitempty"`   // flSize
	DateAdded string      `json:"date_added,omitempty"`  // flDate (YYYY-MM-DD)
	LibID     int64       `json:"lib_id,omitempty"`      // flLibID
	Deleted   bool        `json:"deleted,omitempty"`     // flDeleted
	LibRate   int         `json:"lib_rate,omitempty"`    // flLibRate
	Series    *SeriesInfo `json:"series,omitempty"`      // flSeries + flSerNo
	Keywords  []string    `json:"keywords,omitempty"`    // flKeyWords
}

// SeriesInfo represents series information
type SeriesInfo struct {
	ID       int64  `json:"series_id,omitempty"`
	Name     string `json:"name,omitempty"`
	SeriesNo int    `json:"series_no,omitempty"`
}

// Genre represents a genre (for queries)
type Genre struct {
	ID   int64  `json:"genre_id"`
	Name string `json:"name"`
}

// Keyword represents a keyword (for queries)
type Keyword struct {
	ID   int64  `json:"keyword_id"`
	Name string `json:"name"`
}

type Storager interface {
	AddBook(*Book) error
	Search() error
	List() error
}
