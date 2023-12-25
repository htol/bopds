package scanner

import (
	"encoding/xml"
	"reflect"
	"strings"
	"testing"

	"github.com/htol/bopds/book"
)

func BenchmarkScanLibrary(b *testing.B) {
	if err := ScanLibrary("../lib"); err != nil {
		b.Error(err)
	}
}

func TestParseInpEntry(t *testing.T) {
	entrySeparator := []rune{4}

	s := []string{"Амнуэль,Песах,Рафаэлович:Леонидов,Роман,Леонидович:",
		"sf:", "Престиж Небесной империи", "", "", "1310", "8962", "1310", "1",
		"fb2", "2007-06-20", "ru", "", "", ""}
	s1 := strings.Split("Абеляр,Пьер,:sci_philosophy:История моих бедствий125954261251fb22007-06-29ru", string(entrySeparator))
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

	bookEntry1 := parseInpEntry(s1)
	t.Logf("%#v", bookEntry1)
}
