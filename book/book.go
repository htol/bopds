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
}

type Storager interface {
	AddBook(*Book) error
	Search() error
	List() error
}
