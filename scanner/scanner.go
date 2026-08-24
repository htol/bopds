package scanner

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
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

// inpFieldSep is the INPX field separator (ASCII EOT).
const inpFieldSep = "\x04"

// ScanLibraries discovers libraries under each root and scans them into storage.
// A library is a first-level subdirectory of a root.
func ScanLibraries(roots []string, storage Storager, batchSize int) error {
	libraries, err := DiscoverLibraries(roots)
	if err != nil {
		return err
	}
	return scanAll(libraries, storage, batchSize)
}

// ScanLibrary scans a single discovered library into storage.
func ScanLibrary(lib Library, storage Storager, batchSize int) error {
	return scanAll([]Library{lib}, storage, batchSize)
}

func scanAll(libraries []Library, storage Storager, batchSize int) error {
	entries := make(chan *book.Book)

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		defer func() {
			close(entries)
		}()
		for _, lib := range libraries {
			if err := scanLibrary(ctx, lib, entries); err != nil {
				return fmt.Errorf("scan library %s: %w", lib.Name, err)
			}
		}
		return nil
	})

	g.Go(func() error {
		if batchSize <= 0 {
			batchSize = 1000
		}
		batch := make([]*book.Book, 0, batchSize)

		for entry := range entries {
			batch = append(batch, entry)
		if len(batch) >= batchSize {
				if err := storage.AddBatch(batch); err != nil {
					return fmt.Errorf("failed to add batch: %w", err)
				}
				// Keep capacity, reset length
				batch = batch[:0]
			}
		}
		if len(batch) > 0 {
			if err := storage.AddBatch(batch); err != nil {
				return fmt.Errorf("failed to add batch: %w", err)
			}
		}
		return nil
	})

	return g.Wait()
}

// scanLibrary scans one library: loose .inp files and .inpx indexes.
func scanLibrary(ctx context.Context, lib Library, entries chan<- *book.Book) error {
	var (
		files []string
		inps  []string
		inpxs []string
	)

	exts := map[string]bool{
		".fb2": true,
		".zip": true,
		".7z":  true,
	}

	// archives maps an archive filename to its path relative to the library root.
	archives := map[string]string{}

	err := filepath.WalkDir(lib.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if !d.IsDir() && exts[ext] {
			files = append(files, path)
		}

		switch ext {
		case ".inp":
			inps = append(inps, path)
		case ".inpx":
			inpxs = append(inpxs, path)
		case ".zip", ".7z":
			name := filepath.Base(path)
			if existing, dup := archives[name]; dup {
				logger.Warn("Duplicate archive name in library, keeping the first", "library", lib.Name, "kept", existing, "ignored", path)
				return nil
			}
			rel, err := filepath.Rel(lib.Path, path)
			if err != nil {
				return err
			}
			archives[name] = rel
		}

		return nil
	})

	if err != nil {
		return err
	}

	if len(inpxs) > 0 {
		logger.Info("Present indexes", "files", inpxs)
		if err = checkInpxFiles(ctx, lib, inpxs, archives, entries); err != nil {
			return fmt.Errorf("scan inpx: %w", err)
		}
	}

	for _, inp := range inps {
		if err := processLooseInp(ctx, lib, inp, archives, entries); err != nil {
			return fmt.Errorf("scan inp %s: %w", inp, err)
		}
	}

	return nil
}

// lookupArchive resolves the companion archive for baseName. An archive in
// sourceDir (the directory of the index source) wins over an archive
// anywhere else in the library; at the same level .zip beats .7z. The
// directory tier keeps same-named non-book archives in sibling directories
// (e.g. cover-image zips) from shadowing the real companion archive.
func lookupArchive(archives map[string]string, sourceDir, baseName string) (string, bool) {
	for _, ext := range []string{".zip", ".7z"} {
		if rel, ok := archives[baseName+ext]; ok && filepath.Dir(rel) == sourceDir {
			return rel, true
		}
	}
	for _, ext := range []string{".zip", ".7z"} {
		if rel, ok := archives[baseName+ext]; ok {
			return rel, true
		}
	}
	return "", false
}

// processLooseInp imports one loose .inp file; entries without a companion archive are skipped.
func processLooseInp(ctx context.Context, lib Library, path string, archives map[string]string, entries chan<- *book.Book) error {
	baseName := strings.TrimSuffix(filepath.Base(path), ".inp")
	rel, err := filepath.Rel(lib.Path, path)
	if err != nil {
		return fmt.Errorf("resolve inp path %s: %w", path, err)
	}
	archiveRel, ok := lookupArchive(archives, filepath.Dir(rel), baseName)
	if !ok {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open inp %s: %w", path, err)
	}
	defer f.Close()

	return sendInpLines(ctx, f, func(b *book.Book) {
		b.Archive = archiveRel
		b.Library = lib.Name
	}, entries)
}

// sendInpLines parses separator-delimited .inp lines from r, applies fn to each
// entry, and sends entries with a title on the channel.
func sendInpLines(ctx context.Context, r io.Reader, fn func(*book.Book), entries chan<- *book.Book) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		bookEntry := parseInpEntry(strings.Split(line, inpFieldSep))
		fn(bookEntry)
		if bookEntry.Title != "" {
			select {
			case entries <- bookEntry:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan inp: %w", err)
	}
	return nil
}

func checkInpxFiles(ctx context.Context, lib Library, files []string, archives map[string]string, entries chan<- *book.Book) error {
	for _, file := range files {
		inpxRel, err := filepath.Rel(lib.Path, file)
		if err != nil {
			return fmt.Errorf("resolve inpx path %s: %w", file, err)
		}
		sourceDir := filepath.Dir(inpxRel)

		arch, err := zip.OpenReader(file)
		if err != nil {
			return fmt.Errorf("open zip %s: %w", file, err)
		}

		for _, archiveEntry := range arch.File {
			if !strings.HasSuffix(archiveEntry.Name, ".inp") {
				continue
			}

			// don't scan inp if companion archive is absent anywhere in the library
			baseName := strings.TrimSuffix(archiveEntry.Name, ".inp")
			archiveRel, ok := lookupArchive(archives, sourceDir, baseName)
			if !ok {
				continue
			}

			logger.Info("Processing archive", "library", lib.Name, "file", archiveRel)
			startTime := time.Now()

			content, err := archiveEntry.Open()
			if err != nil {
				arch.Close()
				return fmt.Errorf("failed to read file in zip %s: %w", archiveEntry.Name, err)
			}

			err = sendInpLines(ctx, content, func(b *book.Book) {
				b.Archive = archiveRel
				b.Library = lib.Name
			}, entries)
			content.Close()
			if err != nil {
				arch.Close()
				return err
			}
			logger.Info("Finished processing archive", "library", lib.Name, "file", archiveRel, "duration", time.Since(startTime))
		}
		arch.Close()
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
