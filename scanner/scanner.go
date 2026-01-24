package scanner

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/htol/bopds/book"
	"golang.org/x/sync/errgroup"
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

type Storager interface {
	Add(*book.Book) error
	AddBatch([]*book.Book) error
}

// ScanLibrary scanning all file names in libraries directories
func ScanLibrary(basedir string, storage Storager, batchSize int) error {
	var (
		files []string
		inpxs []string
	)

	exts := map[string]bool{
		".fb2": true,
		".zip": true,
		".7z":  true,
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

	wg := sync.WaitGroup{}
	entries := make(chan *book.Book)

	g, ctx := errgroup.WithContext(context.Background())

	wg.Add(1)
	g.Go(func() error {
		defer wg.Done()
		if len(inpxs) > 0 {
			log.Println("Present indexes: ", inpxs)
			if err = checkInpxFiles(ctx, basedir, inpxs, entries); err != nil {
				return err
			}
		}
		return nil
	})

	wg.Add(1)
	go func() {
		defer wg.Done()
		if batchSize <= 0 {
			batchSize = 1000
		}
		batch := make([]*book.Book, 0, batchSize)

		for entry := range entries {
			batch = append(batch, entry)
			if len(batch) >= batchSize {
				if err := storage.AddBatch(batch); err != nil {
					log.Printf("failed to add batch: %v", err)
				}
				// Keep capacity, reset length
				batch = batch[:0]
			}
		}
		if len(batch) > 0 {
			if err := storage.AddBatch(batch); err != nil {
				log.Printf("failed to add batch: %v", err)
			}
		}
	}()

	wg.Wait()

	return nil
}

func checkInpxFiles(ctx context.Context, basedir string, files []string, entries chan<- *book.Book) error {
	defer close(entries)

	for _, file := range files {
		arch, err := zip.OpenReader(file)
		if err != nil {
			return fmt.Errorf("open zip %s: %w", file, err)
		}
		defer arch.Close()

		for _, archiveEntry := range arch.File {
			if !strings.HasSuffix(archiveEntry.Name, ".inp") {
				continue
			}

			// don't scan inp if library archive absent
			// Check for both .zip and .7z archives
			baseName := strings.TrimSuffix(archiveEntry.Name, ".inp")
			libArchiveFile := filepath.Join(basedir, baseName+".zip")
			if _, err := os.Stat(libArchiveFile); errors.Is(err, os.ErrNotExist) {
				// Try .7z if .zip not found
				libArchiveFile = filepath.Join(basedir, baseName+".7z")
				if _, err := os.Stat(libArchiveFile); errors.Is(err, os.ErrNotExist) {
					continue
				}
			}

			log.Printf("Processing archive: %s", libArchiveFile)
			startTime := time.Now()

			content, err := archiveEntry.Open()
			if err != nil {
				log.Printf("Failed to read %s in zip: %s", archiveEntry.Name, err)
				continue
			}
			defer content.Close()

			scanner := bufio.NewScanner(content)
			fieldSeparator := []rune{4}
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				inpEntry := strings.Split(line, string(fieldSeparator))
				bookEntry := parseInpEntry(inpEntry)
				bookEntry.Archive = libArchiveFile
				if bookEntry.Title != "" {
					select {
					case entries <- bookEntry:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
			if err := scanner.Err(); err != nil {
				log.Printf("Scanner error on %s: %s", archiveEntry.Name, err)
			}
			log.Printf("Finished processing %s in %v", libArchiveFile, time.Since(startTime))
		}
	}
	return nil
}

func parseInpEntry(entry []string) *book.Book {
	const (
		listSep = ":"
		itemSep = ","
	)
	bookEntry := &book.Book{
		Deleted: false, // Default: present/active (0 in INPX)
	}

	for fieldIdx, field := range entry {
		switch fieldIdx {
		case flAuthor:
			if len(field) == 0 {
				break
			}
			list := strings.Split(field[:len(field)-1], listSep)
			for _, entry := range list {
				parts := strings.Split(entry, itemSep)
				if len(parts) >= 3 {
					author := &book.Author{
						FirstName:  parts[1],
						MiddleName: parts[2],
						LastName:   parts[0],
					}
					bookEntry.Author = append(bookEntry.Author, *author)
				}
			}

		case flGenre:
			if len(field) == 0 {
				break
			}
			genres := strings.Split(field[:len(field)-1], listSep)
			bookEntry.Genres = genres

		case flTitle:
			bookEntry.Title = field

		case flSeries:
			if field != "" {
				if bookEntry.Series == nil {
					bookEntry.Series = &book.SeriesInfo{}
				}
				bookEntry.Series.Name = field
			}

		case flSerNo:
			if field != "" {
				if bookEntry.Series == nil {
					bookEntry.Series = &book.SeriesInfo{}
				}
				if serNo, err := strconv.Atoi(field); err == nil {
					bookEntry.Series.SeriesNo = serNo
				}
			}

		case flFile:
			bookEntry.FileName = field

		case flSize:
			if field != "" {
				if size, err := strconv.ParseInt(field, 10, 64); err == nil {
					bookEntry.FileSize = size
				}
			}

		case flLibID:
			if field != "" {
				if libID, err := strconv.ParseInt(field, 10, 64); err == nil {
					bookEntry.LibID = libID
				}
			}

		case flDeleted:
			if field != "" {
				// INPX: 0=present/active, 1=marked for deletion or absent
				if deleted, err := strconv.Atoi(field); err == nil {
					bookEntry.Deleted = (deleted == 1)
				}
			}

		case flExt:
			bookEntry.FileName += "." + field

		case flDate:
			bookEntry.DateAdded = field

		case flLang:
			bookEntry.Lang = field

		case flLibRate:
			if field != "" {
				if rate, err := strconv.Atoi(field); err == nil {
					bookEntry.LibRate = rate
				}
			}

		case flKeyWords:
			if field != "" {
				bookEntry.Keywords = parseKeywords(field)
			}

		case flURI:
			// Deprecated, ignore

		default:
		}
	}
	return bookEntry
}

// Helper function to parse keywords
func parseKeywords(field string) []string {
	trimmed := strings.TrimSpace(field)
	if trimmed == "" {
		return []string{}
	}

	var parts []string
	if strings.Contains(trimmed, ",") {
		parts = strings.Split(trimmed, ",")
	} else {
		parts = strings.Fields(trimmed)
	}

	var result []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
