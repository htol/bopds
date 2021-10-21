package scanner

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html/charset"
)

type Author struct {
	XMLName   xml.Name `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 author"`
	FirstName string   `xml:"first-name"`
	LastName  string   `xml:"last-name"`
}

type Book struct {
	XMLName xml.Name `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 title-info"`
	Author  Author   `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 author"`
	Title   string   `xml:"http://www.gribuser.ru/xml/fictionbook/2.0 book-title"`
	Lang    string   `xml:"lang"`
}

// ScanLibrary scanning all file names in libraries directories
func ScanLibrary(basedir string) error {
	var files []string

	exts := map[string]bool{
		".fb2": true,
		".zip": true,
	}

	err := filepath.Walk(basedir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && exts[filepath.Ext(path)] {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// fmt.Println(files)

	for idx, file := range files {
		fmt.Printf("Working on file: %d %s\n", idx, file)
		if strings.HasSuffix(file, ".zip") {
			log.Println("archive found")
			arch, err := zip.OpenReader(file)
			if err != nil {
				log.Fatalf("Failed to open: %s", err)
			}
			defer arch.Close()

			for _, entry := range arch.File {
				log.Printf("entry: %+v", entry.Name)
				content, err := entry.Open()
				if err != nil {
					log.Printf("Failed to read %s in zip: %s", entry.Name, err)
					continue
				}
				defer content.Close()

				if err = bookReader(content); err != nil {
					log.Printf("fail to read book %s", err)
				}
			}

		} else if strings.HasSuffix(file, ".fb2") {
			// TODO: check if it's zipped

			book, err := os.Open(file)
			if err != nil {
				return err
			}
			defer book.Close()

			err = bookReader(book)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func bookReader(book io.ReadCloser) error {
	decoder := xml.NewDecoder(book)
	decoder.CharsetReader = charset.NewReaderLabel

	// TODO: have to detect file content xml in fb2, zip with fb2 files or zip in fb2 file before loop
	var b Book

	for t, err := decoder.Token(); t != nil; t, err = decoder.Token() {
		if err != nil {
			return err
		}

		switch se := t.(type) {
		case xml.StartElement:
			// fmt.Printf("s: %+v\n", se.Name.Local)
			if se.Name.Local == "title-info" {
				err = decoder.DecodeElement(&b, &se)
				if err != nil {
					return err
				}
			}

		case xml.EndElement:
			// fmt.Printf("e: %+v\n\n", se.Name.Local)
			break

		default:
			//fmt.Printf("d: %+v\n\n", se)
		}
	}

	if len(b.Title) == 0 {
		fmt.Println("   ---   Title not found")
		return nil
	}

	// fmt.Printf("reuslt: %+v\n", b)
	fmt.Printf("Autor: %s %s, Title: %s, Lang: %s\n",
		b.Author.FirstName,
		b.Author.LastName,
		b.Title,
		b.Lang)

	return nil
}
