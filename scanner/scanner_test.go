package scanner

import (
	"encoding/xml"
	"reflect"
	"testing"

	"github.com/htol/bopds/book"
)

func BenchmarkScanLibrary(b *testing.B) {
	if err := ScanLibrary("../lib"); err != nil {
		b.Error(err)
	}
}

func TestParseInpEntry(t *testing.T) {

	s := []string{"Амнуэль,Песах,Рафаэлович:Леонидов,Роман,Леонидович:",
		"sf:", "Престиж Небесной империи", "", "", "1310", "8962", "1310", "1",
		"fb2", "2007-06-20", "ru", "", "", ""}
	expecting := &book.Book{
		XMLName: xml.Name{Space: "", Local: ""},
		Author: []book.Author{
			book.Author{XMLName: xml.Name{Space: "", Local: ""}, FirstName: "Песах", MiddleName: "Рафаэлович", LastName: "Амнуэль"},
			book.Author{XMLName: xml.Name{Space: "", Local: ""}, FirstName: "Роман", MiddleName: "Леонидович", LastName: "Леонидов"}},
		Title: "Престиж Небесной империи", Lang: "ru", Genres: []string{"sf"}, Archive: "", FileName: "1310.fb2"}

	bookEntry := parseInpEntry(s)
	if !reflect.DeepEqual(bookEntry, expecting) {
		t.Logf("%#v", bookEntry)
		t.Logf("%#v", expecting)
		t.Error("inp entry parsed incorrectly")
	}
}
