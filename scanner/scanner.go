package scanner

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/repo"
	"golang.org/x/net/html/charset"
)

const (
	flAuthor = iota
	flGenre
	flTitle
	flSeries
	flSerNo
	flFile
	flSize
	flLibID
	flDeleted
	flExt
	flDate
	flLang
	flLibRate
	flKeyWords
	flURI // depricated?
)

// ScanLibrary scanning all file names in libraries directories
func ScanLibrary(basedir string) error {
	var (
		files []string
		inpxs []string
	)

	exts := map[string]bool{
		".fb2": true,
		".zip": true,
	}

	err := filepath.WalkDir(basedir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && exts[filepath.Ext(path)] {
			files = append(files, path)
		}
		if !d.IsDir() && (filepath.Ext(path) == ".inpx") {
			inpxs = append(inpxs, path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	if len(inpxs) > 0 {
		log.Println("Present indexes: ", inpxs)
		if err = checkInpxFiles(basedir, inpxs); err != nil {
			return err
		}
	}

	//if err = checkFilesContent(files); err != nil {
	//	return err
	//}

	return nil
}

func checkInpxFiles(basedir string, files []string) error {
	db := repo.GetStorage("books.db")
	defer db.Close()

	for _, file := range files {
		arch, err := zip.OpenReader(file)
		if err != nil {
			log.Fatalf("Failed to open: %s", err)
		}
		defer arch.Close()

		for _, archiveEntry := range arch.File {
			if !strings.HasSuffix(archiveEntry.Name, ".inp") {
				continue
			}

			// don't scan inp if library archive absent
			// TODO: return archive names present in index
			libArchiveFile := filepath.Join(basedir, strings.TrimSuffix(archiveEntry.Name, ".inp")+".zip")
			if _, err := os.Stat(libArchiveFile); errors.Is(err, os.ErrNotExist) {
				continue
			}

			content, err := archiveEntry.Open()
			if err != nil {
				log.Printf("Failed to read %s in zip: %s", archiveEntry.Name, err)
				continue
			}
			defer content.Close()

			scanner := bufio.NewScanner(content)
			entrySeparator := []rune{4}
			for scanner.Scan() {
				inpEntry := strings.Split(scanner.Text(), string(entrySeparator))
				//fmt.Printf("%#v\n", inpEntry)
				bookEntry := parseInpEntry(inpEntry)
				bookEntry.Archive = libArchiveFile
				//fmt.Printf("%#v\n", bookEntry)
				db.Add(bookEntry)
			}

		}
	}
	return nil
}

func parseInp(reader *io.ReadCloser) error {

	return nil
}

func parseInpEntry(entry []string) *book.Book {
	const (
		listSep = ":"
		itemSep = ","
	)
	bookEntry := &book.Book{}
	for fieldIdx, field := range entry {
		switch fieldIdx {
		case flAuthor:
			list := strings.Split(field[:len(field)-1], listSep)
			//fmt.Printf("list: %#v\n", list)
			for _, entry := range list {
				parts := strings.Split(entry, itemSep)
				author := &book.Author{
					FirstName:  parts[1],
					MiddleName: parts[2],
					LastName:   parts[0],
				}
				bookEntry.Author = append(bookEntry.Author, *author)
			}
			// fmt.Printf("Author: %#v\n", book.Author)
		case flGenre:
			genres := strings.Split(field[:len(field)-1], listSep)
			//fmt.Printf("list: %#v\n", genres)
			bookEntry.Genres = genres
		case flTitle:
			bookEntry.Title = field
		case flSeries:
			//fmt.Println(field)
		case flSerNo:
			// probably seq number (tome number) in series
			//fmt.Println(field)
		case flFile:
			bookEntry.FileName = field
		case flSize:
			//file size
		case flLibID:
			// was equal to filename number
		case flDeleted:
			// TODO: need further investigation
			//fmt.Println(field)
		case flExt:
			bookEntry.FileName += "." + field
		case flDate:
		case flLang:
			bookEntry.Lang = field
		case flLibRate:
		case flKeyWords:
		case flURI: // depricated?
		default:
		}
	}
	return bookEntry
}

func checkFilesContent(files []string) error {
	for _, file := range files {
		//fmt.Printf("Working on file: %d %s\n", idx, file)
		if strings.HasSuffix(file, ".zip") {
			//log.Println("archive found")
			arch, err := zip.OpenReader(file)
			if err != nil {
				log.Fatalf("Failed to open: %s", err)
			}
			defer arch.Close()

			for _, entry := range arch.File {
				//log.Printf("entry: %+v", entry.Name)
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

func bookReader(bookContent io.ReadCloser) error {
	decoder := xml.NewDecoder(bookContent)
	decoder.CharsetReader = charset.NewReaderLabel

	// TODO: have to detect file content xml in fb2, zip with fb2 files or zip in fb2 file before loop
	var b book.Book

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
	//	fmt.Printf("Autor: %s %s, Title: %s, Lang: %s\n",
	//		b.Author.FirstName,
	//		b.Author.LastName,
	//		b.Title,
	//		b.Lang)

	return nil
}
