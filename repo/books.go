package repo

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/config"
	"github.com/htol/bopds/logger"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func escapeFTS5Query(query string) string {
	query = strings.TrimSpace(query)

	space := regexp.MustCompile(`\s+`)
	query = space.ReplaceAllString(query, " ")
	query = strings.TrimSpace(query)

	escaped := strings.ReplaceAll(query, "\"", "\"\"")
	escaped = strings.ReplaceAll(escaped, "-", " ")
	escaped = strings.ReplaceAll(escaped, "'", "")
	escaped = strings.ReplaceAll(escaped, "(", "")
	escaped = strings.ReplaceAll(escaped, ")", "")
	escaped = strings.ReplaceAll(escaped, "{", "")
	escaped = strings.ReplaceAll(escaped, "}", "")

	return strings.TrimSpace(escaped)
}

type Repo struct {
	db   *sql.DB
	path string
	// Cache for fast import
	mu           sync.RWMutex
	authorCache  map[string]int64
	genreCache   map[string]int64
	seriesCache  map[string]int64
	keywordCache map[string]int64
}

type linkData struct {
	bookID  int64
	otherID int64
}

// AuthorKey struct for map key
type AuthorKey struct {
	Last   string
	First  string
	Middle string
}

var _ Repository = (*Repo)(nil)

func GetStorage(path string) *Repo {
	return GetStorageWithConfig(path, config.Load())
}

func GetStorageWithConfig(path string, cfg *config.Config) *Repo {
	r := &Repo{
		path:         path,
		authorCache:  make(map[string]int64),
		genreCache:   make(map[string]int64),
		seriesCache:  make(map[string]int64),
		keywordCache: make(map[string]int64),
	}

	db, err := sql.Open("sqlite3", "file:"+r.path+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		logger.Error("Failed to open database", "path", r.path, "error", err)
		panic(err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	// Performance optimizations
	// 1. Memory Mapped I/O: Map up to 30GB. If file is smaller, maps entire file.
	if _, err := db.Exec("PRAGMA mmap_size = 30000000000"); err != nil {
		logger.Warn("Failed to set mmap_size", "error", err)
	}

	// 2. Cache Size: -64000 means 64MB of cache. Positive would be N pages.
	// 64MB is a safe conservative default.
	if _, err := db.Exec("PRAGMA cache_size = -64000"); err != nil {
		logger.Warn("Failed to set cache_size", "error", err)
	}

	// 3. Temporary Store: Use RAM for temp tables/indices (critical for sorting during CREATE INDEX)
	if _, err := db.Exec("PRAGMA temp_store = MEMORY"); err != nil {
		logger.Warn("Failed to set temp_store", "error", err)
	}

	r.db = db

	// Create tables first (without indexes for authors/books to allow fast bulk insert option)
	// We still need uniqueness constraints on core tables if we rely on them for logic,
	// but pure performance indexes can be managed separately.
	// Actually, for safety, let's keep the CREATE TABLE + UNIQUE constraints in CreateIndexes or here?
	// The problem is if we drop indexes that back UNIQUE constraints, we lose the constraint.
	// SQLite: "Books" table doesn't have unique constraints other than PK.
	// "Authors" has UNIQUE(first, middle, last). If we drop that, we might get duplicates if logic fails.
	// However, the user wants SPEED.
	// Safe approach: Drop only non-unique performance indexes.

	if err := r.CreateIndexes(); err != nil {
		logger.Error("Failed to create schema/indexes", "error", err)
		panic(err)
	}

	// Recreate triggers to ensure they are up-to-date
	// Note: We only keep the basic insert/delete triggers for FTS.
	// Author, series, and genre linking is handled by RebuildFTSIndex() after bulk imports.
	triggerStmt := `
           DROP TRIGGER IF EXISTS books_fts_insert;
           CREATE TRIGGER books_fts_insert AFTER INSERT ON books BEGIN
               INSERT INTO books_fts(title, author, series, genre, book_id)
               VALUES (
                 new.title,
                 NULL,  -- Author will be populated by RebuildFTSIndex
                 NULL,  -- Series will be populated by RebuildFTSIndex
                 NULL,  -- Genre will be populated by RebuildFTSIndex
                 new.book_id
               );
            END;

           DROP TRIGGER IF EXISTS books_fts_delete;
           CREATE TRIGGER books_fts_delete AFTER DELETE ON books BEGIN
               DELETE FROM books_fts WHERE book_id = old.book_id;
           END;

           -- Drop old triggers that caused performance issues during bulk import
           DROP TRIGGER IF EXISTS books_fts_authors_insert;
           DROP TRIGGER IF EXISTS books_fts_authors_delete;
           DROP TRIGGER IF EXISTS books_fts_series_insert;
           DROP TRIGGER IF EXISTS books_fts_series_delete;
           DROP TRIGGER IF EXISTS books_fts_genres_insert;
           DROP TRIGGER IF EXISTS books_fts_genres_delete;
    `
	_, err = db.Exec(triggerStmt)
	if err != nil {
		logger.Error("Failed to update triggers", "error", err)
		panic(err)
	}

	r.migrateAddTranslitName()
	r.SyncGenreDisplayNames()

	return r
}

func (r *Repo) CreateIndexes() error {
	sqlStmt := `
           CREATE TABLE IF NOT EXISTS "authors" (
               author_id integer primary key autoincrement not null,
               first_name text,
               middle_name text,
               last_name text,
               UNIQUE(first_name, middle_name, last_name)
           );
           CREATE INDEX IF NOT EXISTS [I_first_name] ON "authors" ([first_name]);
           CREATE INDEX IF NOT EXISTS [I_last_name] ON "authors" ([last_name]);
           CREATE INDEX IF NOT EXISTS [I_middle_name] ON "authors" ([middle_name]);

           CREATE TABLE IF NOT EXISTS "books" (
                book_id integer primary key autoincrement not null,
                title text,
                lang text,
                archive text,
                filename text,
                file_size integer,
                date_added text,
                lib_id integer,
                deleted boolean default 0, -- 0=present/active, 1=marked for deletion or absent
                lib_rate integer
            );
           CREATE INDEX IF NOT EXISTS [I_title] ON "books" ([title]);

           CREATE TABLE IF NOT EXISTS "book_authors" (
               book_id INTEGER NOT NULL,
               author_id INTEGER NOT NULL,
               FOREIGN KEY (book_id) REFERENCES books(book_id),
               FOREIGN KEY (author_id) REFERENCES authors(author_id)
           );
           CREATE INDEX IF NOT EXISTS [I_book_id] ON "book_authors" ([book_id]);
           CREATE INDEX IF NOT EXISTS [I_author_id] ON "book_authors" ([author_id]);

           CREATE TABLE IF NOT EXISTS "genres" (
               genre_id integer primary key autoincrement not null,
               name text unique not null,
               display_name text
           );

           CREATE TABLE IF NOT EXISTS "book_genres" (
               book_id INTEGER NOT NULL,
               genre_id INTEGER NOT NULL,
               PRIMARY KEY (book_id, genre_id),
               FOREIGN KEY (book_id) REFERENCES books(book_id),
               FOREIGN KEY (genre_id) REFERENCES genres(genre_id)
           );

           CREATE TABLE IF NOT EXISTS "series" (
               series_id INTEGER PRIMARY KEY AUTOINCREMENT,
               name TEXT UNIQUE NOT NULL
           );

           CREATE TABLE IF NOT EXISTS "book_series" (
               book_id INTEGER NOT NULL,
               series_id INTEGER NOT NULL,
               series_no INTEGER,
               PRIMARY KEY (book_id, series_id),
               FOREIGN KEY (book_id) REFERENCES books(book_id) ON DELETE CASCADE,
               FOREIGN KEY (series_id) REFERENCES series(series_id)
           );
           CREATE INDEX IF NOT EXISTS [idx_book_series_book_id] ON [book_series] ([book_id]);
           CREATE INDEX IF NOT EXISTS [idx_book_series_series_id] ON [book_series] ([series_id]);

           CREATE TABLE IF NOT EXISTS "keywords" (
               keyword_id INTEGER PRIMARY KEY AUTOINCREMENT,
               name TEXT UNIQUE NOT NULL
           );

           CREATE TABLE IF NOT EXISTS "book_keywords" (
               book_id INTEGER NOT NULL,
               keyword_id INTEGER NOT NULL,
               PRIMARY KEY (book_id, keyword_id),
               FOREIGN KEY (book_id) REFERENCES books(book_id) ON DELETE CASCADE,
               FOREIGN KEY (keyword_id) REFERENCES keywords(keyword_id)
           );
           CREATE INDEX IF NOT EXISTS [idx_book_keywords_book_id] ON [book_keywords] ([book_id]);
           CREATE INDEX IF NOT EXISTS [idx_book_keywords_keyword_id] ON [book_keywords] ([keyword_id]);

           CREATE VIRTUAL TABLE IF NOT EXISTS books_fts USING fts5(title, author, series, genre, book_id);
  	    `
	_, err := r.db.Exec(sqlStmt)
	return err
}

func (r *Repo) DropIndexes() error {
	// Drop non-unique performance indexes
	sqlStmt := `
		DROP INDEX IF EXISTS I_first_name;
		DROP INDEX IF EXISTS I_last_name;
		DROP INDEX IF EXISTS I_middle_name;
		DROP INDEX IF EXISTS I_title;
		DROP INDEX IF EXISTS I_book_id;
		DROP INDEX IF EXISTS I_author_id;
		DROP INDEX IF EXISTS idx_book_series_book_id;
		DROP INDEX IF EXISTS idx_book_series_series_id;
		DROP INDEX IF EXISTS idx_book_keywords_book_id;
		DROP INDEX IF EXISTS idx_book_keywords_keyword_id;
	`
	_, err := r.db.Exec(sqlStmt)
	return err
}

func (r *Repo) Close() error {
	return r.db.Close()
}

// Ping checks if database connection is alive
func (r *Repo) Ping() error {
	return r.db.Ping()
}

func (r *Repo) InitCache() error {
	logger.Info("Initializing in-memory cache...")
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load Authors
	rows, err := r.db.Query("SELECT author_id, first_name, middle_name, last_name FROM authors")
	if err != nil {
		return fmt.Errorf("load authors: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		var f, m, l string
		if err := rows.Scan(&id, &f, &m, &l); err != nil {
			return err
		}
		key := fmt.Sprintf("%s|%s|%s", f, m, l)
		r.authorCache[key] = id
		count++
	}
	logger.Info("Loaded authors", "count", count)

	// Load Genres
	rows, err = r.db.Query("SELECT genre_id, name FROM genres")
	if err != nil {
		return fmt.Errorf("load genres: %w", err)
	}
	defer rows.Close()
	count = 0
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		r.genreCache[name] = id
		count++
	}
	logger.Info("Loaded genres", "count", count)

	// Load Series
	rows, err = r.db.Query("SELECT series_id, name FROM series")
	if err != nil {
		return fmt.Errorf("load series: %w", err)
	}
	defer rows.Close()
	count = 0
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		r.seriesCache[name] = id
		count++
	}
	logger.Info("Loaded series", "count", count)

	// Load Keywords
	rows, err = r.db.Query("SELECT keyword_id, name FROM keywords")
	if err != nil {
		return fmt.Errorf("load keywords: %w", err)
	}
	defer rows.Close()
	count = 0
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		r.keywordCache[name] = id
		count++
	}
	logger.Info("Loaded keywords", "count", count)

	return nil
}

func (r *Repo) SetFastMode(enable bool) error {
	var mode string
	if enable {
		mode = "OFF"
	} else {
		mode = "NORMAL" // or FULL
	}
	_, err := r.db.Exec(fmt.Sprintf("PRAGMA synchronous = %s", mode))
	return err
}

// SetBulkImportMode configures the database for bulk import operations
// When enabled: disables WAL auto-checkpoint, increases cache to 256MB
// When disabled: restores normal checkpoint behavior and cache size
func (r *Repo) SetBulkImportMode(enable bool) error {
	if enable {
		// 1. Disable WAL auto-checkpoint to prevent fsync storms during bulk import
		// Value of 0 disables auto-checkpoint entirely
		if _, err := r.db.Exec("PRAGMA wal_autocheckpoint = 0"); err != nil {
			return fmt.Errorf("disable wal_autocheckpoint: %w", err)
		}

		// 2. Increase cache size to 256MB for bulk operations
		// Negative value means size in KiB, so -256000 = ~256MB
		if _, err := r.db.Exec("PRAGMA cache_size = -256000"); err != nil {
			return fmt.Errorf("set cache_size: %w", err)
		}

		logger.Info("Bulk import mode enabled: WAL auto-checkpoint disabled, cache 256MB")
	} else {
		// Restore default WAL auto-checkpoint (1000 pages = ~4MB)
		if _, err := r.db.Exec("PRAGMA wal_autocheckpoint = 1000"); err != nil {
			return fmt.Errorf("enable wal_autocheckpoint: %w", err)
		}

		// Restore normal cache size (64MB)
		if _, err := r.db.Exec("PRAGMA cache_size = -64000"); err != nil {
			return fmt.Errorf("restore cache_size: %w", err)
		}

		logger.Info("Bulk import mode disabled: WAL auto-checkpoint enabled, cache 64MB")
	}
	return nil
}

// CheckpointWAL performs a WAL checkpoint to write all pending changes to the database file
// Use TRUNCATE mode for maximum efficiency: resets WAL file to zero bytes
func (r *Repo) CheckpointWAL() error {
	result, err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("wal_checkpoint: %w", err)
	}
	rows, _ := result.RowsAffected()
	logger.Info("WAL checkpoint completed", "rows_affected", rows)
	return nil
}

// AddBatch adds multiple books in a single transaction
func (r *Repo) AddBatch(records []*book.Book) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logger.Error("Failed to rollback transaction", "error", err)
		}
	}()

	// New Bulk Strategy
	// 1. Insert Books in chunks and get IDs
	if err := r.bulkInsertBooks(tx, records); err != nil {
		return err
	}

	// 2. Prepare link data
	// We need to resolve all Authors/Genres/Series IDs using the Cache first
	// Because we need the BookID (which we just got) and the EntityID (from cache)
	var bookAuthors []linkData
	var bookGenres []linkData
	var bookKeywords []linkData

	// Create "missing" entries in cache if needed (though cache should have them if we pre-filled? No, un-cached items need insert)
	// Actually, getOrCreate... is still needed for new items.
	// Optimization: We can do a pass to gather all unique new authors/genres, insert them in bulk, update cache.
	// BUT, for now, let's just stick to the cache check we implemented in BatchInserter, but make it bulk-friendly?
	// The current BatchInserter `getOrCreate` logic is fine for "getting IDs".
	// The bottleneck is the INSERT into book_authors.

	// Let's use a temporary BatchInserter just for resolving IDs efficiently if we want,
	// or move getOrCreate logic to Repo methods.
	// For simplicity and speed:
	// We iterate records, resolve IDs (using cache or single insert for new items - new items are rare in subsequent scans,
	// but common in first scan. Ideally we bulk insert new authors too, but that's complex).
	// Let's stick to: Resolve IDs (fast cache hit usually), collect pairs, then BULK INSERT the pairs.

	bi, err := r.NewBatchInserter(tx)
	if err != nil {
		return err
	}
	bi.repo = r
	defer bi.Close()

	for _, b := range records {
		// Resolve Authors
		aIds, err := bi.getOrCreateAuthors(b.Author)
		if err != nil {
			return err
		}
		for _, aID := range aIds {
			bookAuthors = append(bookAuthors, linkData{b.BookID, aID})
		}

		// Resolve Genes
		for _, gName := range b.Genres {
			if gName == "" {
				continue
			}
			gID, err := bi.getOrCreateGenre(gName)
			if err != nil {
				return err
			}
			bookGenres = append(bookGenres, linkData{b.BookID, gID})
		}

		// Resolve Series
		// Handled by bulkInsertSeriesLinks later to avoid duplicate getOrCreate logic

		// Resolve Keywords
		for _, kw := range b.Keywords {
			if kw == "" {
				continue
			}
			kID, err := bi.getOrCreateKeyword(kw)
			if err != nil {
				return err
			}
			bookKeywords = append(bookKeywords, linkData{b.BookID, kID})
		}
	}

	// 3. Bulk Insert Links
	if err := r.bulkInsertLinks(tx, "book_authors", "book_id, author_id", bookAuthors); err != nil {
		return err
	}
	if err := r.bulkInsertLinks(tx, "book_genres", "book_id, genre_id", bookGenres); err != nil {
		return err
	}
	if err := r.bulkInsertLinks(tx, "book_keywords", "book_id, keyword_id", bookKeywords); err != nil {
		return err
	}

	// Series (needs extra column)
	if err := r.bulkInsertSeriesLinks(tx, records, bi); err != nil {
		return err
	} // Pass bi to resolve series IDs inside

	return tx.Commit()
}

func (r *Repo) bulkInsertBooks(tx *sql.Tx, records []*book.Book) error {
	chunkSize := 3000 // Increased from 100. SQLite limit is usually 32766 params. 3000*9 = 27000. Safe.
	for i := 0; i < len(records); i += chunkSize {
		end := i + chunkSize
		if end > len(records) {
			end = len(records)
		}
		chunk := records[i:end]

		valueStrings := make([]string, 0, len(chunk))
		valueArgs := make([]interface{}, 0, len(chunk)*9)

		for _, b := range chunk {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			del := 0
			if b.Deleted {
				del = 1
			}
			valueArgs = append(valueArgs, b.Title, b.Lang, b.Archive, b.FileName, b.FileSize, b.DateAdded, b.LibID, del, b.LibRate)
		}

		// Use Exec instead of Query with RETURNING - much faster for bulk inserts
		stmt := fmt.Sprintf("INSERT INTO books(title, lang, archive, filename, file_size, date_added, lib_id, deleted, lib_rate) VALUES %s",
			strings.Join(valueStrings, ","))

		result, err := tx.Exec(stmt, valueArgs...)
		if err != nil {
			return err
		}

		// Get the last inserted ID and calculate IDs for all inserted rows
		// SQLite guarantees sequential IDs for multi-value INSERT
		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}

		// Assign IDs to records: lastID is the ID of the last inserted row
		// First row ID = lastID - (len(chunk) - 1)
		firstID := lastID - int64(len(chunk)-1)
		for j := range chunk {
			chunk[j].BookID = firstID + int64(j)
		}
	}
	return nil
}

func (r *Repo) bulkInsertLinks(tx *sql.Tx, table string, columns string, links []linkData) error {
	chunkSize := 10000 // SQLite limit ~32k. 10000 * 2 = 20000 vars. Safe.
	for i := 0; i < len(links); i += chunkSize {
		end := i + chunkSize
		if end > len(links) {
			end = len(links)
		}
		chunk := links[i:end]

		valueStrings := make([]string, 0, len(chunk))
		valueArgs := make([]interface{}, 0, len(chunk)*2)

		for _, l := range chunk {
			valueStrings = append(valueStrings, "(?, ?)")
			valueArgs = append(valueArgs, l.bookID, l.otherID)
		}

		stmt := fmt.Sprintf("INSERT OR IGNORE INTO %s(%s) VALUES %s", table, columns, strings.Join(valueStrings, ","))
		if _, err := tx.Exec(stmt, valueArgs...); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) bulkInsertSeriesLinks(tx *sql.Tx, records []*book.Book, bi *BatchInserter) error {
	type seriesLink struct {
		bID int64
		sID int64
		no  int
	}
	var links []seriesLink

	for _, b := range records {
		if b.Series != nil && b.Series.Name != "" {
			sID, err := bi.getOrCreateSeries(b.Series.Name)
			if err != nil {
				return err
			}
			links = append(links, seriesLink{b.BookID, sID, b.Series.SeriesNo})
		}
	}

	chunkSize := 10000 // 10000 * 3 = 30000 vars. Safe.
	for i := 0; i < len(links); i += chunkSize {
		end := i + chunkSize
		if end > len(links) {
			end = len(links)
		}
		chunk := links[i:end]

		valueStrings := make([]string, 0, len(chunk))
		valueArgs := make([]interface{}, 0, len(chunk)*3)
		for _, l := range chunk {
			valueStrings = append(valueStrings, "(?, ?, ?)")
			valueArgs = append(valueArgs, l.bID, l.sID, l.no)
		}

		stmt := fmt.Sprintf("INSERT OR REPLACE INTO book_series(book_id, series_id, series_no) VALUES %s", strings.Join(valueStrings, ","))
		if _, err := tx.Exec(stmt, valueArgs...); err != nil {
			return err
		}
	}
	return nil
}

// Add adds a single book
func (r *Repo) Add(record *book.Book) error {
	return r.AddBatch([]*book.Book{record})
}

type BatchInserter struct {
	repo                  *Repo
	tx                    *sql.Tx
	stmts                 []*sql.Stmt
	stmtInsertBook        *sql.Stmt
	stmtSelectAuthor      *sql.Stmt
	stmtInsertAuthor      *sql.Stmt
	stmtInsertBookAuthor  *sql.Stmt
	stmtSelectGenre       *sql.Stmt
	stmtInsertGenre       *sql.Stmt
	stmtInsertBookGenre   *sql.Stmt
	stmtSelectSeries      *sql.Stmt
	stmtInsertSeries      *sql.Stmt
	stmtInsertBookSeries  *sql.Stmt
	stmtSelectKeyword     *sql.Stmt
	stmtInsertKeyword     *sql.Stmt
	stmtInsertBookKeyword *sql.Stmt
}

func (r *Repo) NewBatchInserter(tx *sql.Tx) (*BatchInserter, error) {
	bi := &BatchInserter{tx: tx}
	var err error

	prepare := func(query string) (*sql.Stmt, error) {
		s, e := tx.Prepare(query)
		if e != nil {
			return nil, e
		}
		bi.stmts = append(bi.stmts, s)
		return s, nil
	}

	if bi.stmtInsertBook, err = prepare(`INSERT INTO books(title, lang, archive, filename, file_size, date_added, lib_id, deleted, lib_rate) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectAuthor, err = prepare(`SELECT author_id FROM authors WHERE first_name = ? AND middle_name = ? AND last_name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertAuthor, err = prepare(`INSERT INTO authors(first_name, middle_name, last_name) VALUES(?, ?, ?) RETURNING author_id`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookAuthor, err = prepare(`INSERT OR IGNORE INTO book_authors(book_id, author_id) VALUES(?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectGenre, err = prepare(`SELECT genre_id FROM genres WHERE name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertGenre, err = prepare(`INSERT INTO genres(name) VALUES(?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookGenre, err = prepare(`INSERT OR IGNORE INTO book_genres(book_id, genre_id) VALUES(?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectSeries, err = prepare(`SELECT series_id FROM series WHERE name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertSeries, err = prepare(`INSERT INTO series(name) VALUES(?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookSeries, err = prepare(`INSERT OR REPLACE INTO book_series (book_id, series_id, series_no) VALUES (?, ?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtSelectKeyword, err = prepare(`SELECT keyword_id FROM keywords WHERE name = ?`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertKeyword, err = prepare(`INSERT INTO keywords(name) VALUES(?)`); err != nil {
		bi.Close()
		return nil, err
	}
	if bi.stmtInsertBookKeyword, err = prepare(`INSERT OR IGNORE INTO book_keywords (book_id, keyword_id) VALUES (?, ?)`); err != nil {
		bi.Close()
		return nil, err
	}

	return bi, nil
}

func (bi *BatchInserter) Close() {
	for _, s := range bi.stmts {
		s.Close()
	}
}

func (bi *BatchInserter) Add(record *book.Book) error {
	authorIDs, err := bi.getOrCreateAuthors(record.Author)
	if err != nil {
		return fmt.Errorf("get or create author: %w", err)
	}

	deletedInt := 0
	if record.Deleted {
		deletedInt = 1
	}

	res, err := bi.stmtInsertBook.Exec(
		record.Title,
		record.Lang,
		record.Archive,
		record.FileName,
		record.FileSize,
		record.DateAdded,
		record.LibID,
		deletedInt,
		record.LibRate,
	)
	if err != nil {
		return fmt.Errorf("insert book: %w", err)
	}
	bookID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get book id: %w", err)
	}

	for _, authorID := range authorIDs {
		if _, err := bi.stmtInsertBookAuthor.Exec(bookID, authorID); err != nil {
			return fmt.Errorf("link author: %w", err)
		}
	}

	if record.Series != nil && record.Series.Name != "" {
		seriesID, err := bi.getOrCreateSeries(record.Series.Name)
		if err != nil {
			return err
		}
		if _, err := bi.stmtInsertBookSeries.Exec(bookID, seriesID, record.Series.SeriesNo); err != nil {
			return fmt.Errorf("link series: %w", err)
		}
	}

	for _, genre := range record.Genres {
		if genre == "" {
			continue
		}
		genreID, err := bi.getOrCreateGenre(genre)
		if err != nil {
			return err
		}
		if _, err := bi.stmtInsertBookGenre.Exec(bookID, genreID); err != nil {
			return fmt.Errorf("link genre: %w", err)
		}
	}

	for _, kw := range record.Keywords {
		if kw == "" {
			continue
		}
		kwID, err := bi.getOrCreateKeyword(kw)
		if err != nil {
			return err
		}
		if _, err := bi.stmtInsertBookKeyword.Exec(bookID, kwID); err != nil {
			return fmt.Errorf("link keyword: %w", err)
		}
	}

	return nil
}

func (bi *BatchInserter) getOrCreateAuthors(authors []book.Author) ([]int64, error) {
	ids := make([]int64, 0, len(authors))
	for _, a := range authors {
		key := fmt.Sprintf("%s|%s|%s", a.FirstName, a.MiddleName, a.LastName)

		bi.repo.mu.RLock()
		id, ok := bi.repo.authorCache[key]
		bi.repo.mu.RUnlock()

		if ok {
			ids = append(ids, id)
			continue
		}

		// Insert
		err := bi.stmtInsertAuthor.QueryRow(a.FirstName, a.MiddleName, a.LastName).Scan(&id)
		if err != nil {
			return nil, err
		}

		bi.repo.mu.Lock()
		bi.repo.authorCache[key] = id
		bi.repo.mu.Unlock()

		ids = append(ids, id)
	}
	return ids, nil
}

func (bi *BatchInserter) getOrCreateSeries(name string) (int64, error) {
	bi.repo.mu.RLock()
	id, ok := bi.repo.seriesCache[name]
	bi.repo.mu.RUnlock()
	if ok {
		return id, nil
	}

	res, err := bi.stmtInsertSeries.Exec(name)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}

	bi.repo.mu.Lock()
	bi.repo.seriesCache[name] = id
	bi.repo.mu.Unlock()

	return id, nil
}

func (bi *BatchInserter) getOrCreateGenre(name string) (int64, error) {
	bi.repo.mu.RLock()
	id, ok := bi.repo.genreCache[name]
	bi.repo.mu.RUnlock()
	if ok {
		return id, nil
	}

	res, err := bi.stmtInsertGenre.Exec(name)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}

	bi.repo.mu.Lock()
	bi.repo.genreCache[name] = id
	bi.repo.mu.Unlock()

	return id, nil
}

func (bi *BatchInserter) getOrCreateKeyword(name string) (int64, error) {
	bi.repo.mu.RLock()
	id, ok := bi.repo.keywordCache[name]
	bi.repo.mu.RUnlock()
	if ok {
		return id, nil
	}

	res, err := bi.stmtInsertKeyword.Exec(name)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}

	bi.repo.mu.Lock()
	bi.repo.keywordCache[name] = id
	bi.repo.mu.Unlock()

	return id, nil
}

func (r *Repo) Search() error {
	return nil
}

func (r *Repo) List() error {
	return nil
}

func (r *Repo) getOrCreateAuthor(tx *sql.Tx, authors []book.Author) ([]int64, error) {
	if len(authors) < 1 {
		return nil, nil
	}

	result := []int64{}

	selectStmt, err := tx.Prepare(`
		SELECT author_id FROM authors
		WHERE first_name = ? AND middle_name = ? AND last_name = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare select author: %w", err)
	}
	defer selectStmt.Close()

	insertStm, err := tx.Prepare(`
		INSERT INTO authors(first_name, middle_name, last_name) VALUES(?, ?, ?) RETURNING author_id
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert author: %w", err)
	}
	defer insertStm.Close()

	for _, author := range authors {
		var authorID int64

		err := selectStmt.QueryRow(
			author.FirstName,
			author.MiddleName,
			author.LastName,
		).Scan(&authorID)

		if err == sql.ErrNoRows {
			err := insertStm.QueryRow(
				author.FirstName,
				author.MiddleName,
				author.LastName,
			).Scan(&authorID)
			if err != nil {
				return nil, fmt.Errorf("insert author: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("select author: %w", err)
		}

		result = append(result, authorID)
	}
	return result, nil
}

func (r *Repo) GetAuthors() ([]book.Author, error) {
	QUERY := `
		SELECT DISTINCT a.author_id, a.first_name, a.middle_name, a.last_name
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE b.deleted = 0
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query authors: %w", err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author

		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return nil, fmt.Errorf("scan author: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorsByLetter(letters string) ([]book.Author, error) {
	pattern := cases.Title(language.Und, cases.NoLower).String(letters) + "%"
	QUERY := `
		SELECT DISTINCT a.author_id, a.first_name, a.middle_name, a.last_name
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE a.last_name LIKE ? COLLATE NOCASE
		AND b.deleted = 0
		ORDER BY a.last_name
	`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query authors by letter: %w", err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author

		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return nil, fmt.Errorf("scan author by letter: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors by letter: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorsWithBookCount() ([]book.AuthorWithBookCount, error) {
	QUERY := `
		SELECT a.author_id, a.first_name, a.middle_name, a.last_name,
			   COUNT(b.book_id) as book_count
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE b.deleted = 0
		GROUP BY a.author_id, a.first_name, a.middle_name, a.last_name
		ORDER BY a.last_name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query authors with book count: %w", err)
	}
	defer rows.Close()

	authors := make([]book.AuthorWithBookCount, 0)
	for rows.Next() {
		var a book.AuthorWithBookCount
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.BookCount); err != nil {
			return nil, fmt.Errorf("scan author with count: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors with count: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorsWithBookCountByLetter(letters string) ([]book.AuthorWithBookCount, error) {
	pattern := cases.Title(language.Und, cases.NoLower).String(letters) + "%"
	QUERY := `
		SELECT a.author_id, a.first_name, a.middle_name, a.last_name,
			   COUNT(b.book_id) as book_count
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		JOIN books b ON ba.book_id = b.book_id
		WHERE a.last_name LIKE ? COLLATE NOCASE
		AND b.deleted = 0
		GROUP BY a.author_id, a.first_name, a.middle_name, a.last_name
		ORDER BY a.last_name
	`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query authors with book count by letter: %w", err)
	}
	defer rows.Close()

	authors := make([]book.AuthorWithBookCount, 0)
	for rows.Next() {
		var a book.AuthorWithBookCount
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.BookCount); err != nil {
			return nil, fmt.Errorf("scan author with count by letter: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authors with count by letter: %w", err)
	}

	return authors, nil
}

func (r *Repo) GetAuthorByID(id int64) (*book.Author, error) {
	QUERY := `SELECT author_id, first_name, middle_name, last_name FROM authors WHERE author_id = ?`

	var a book.Author
	err := r.db.QueryRow(QUERY, id).Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get author by ID %d: %w", id, err)
	}

	return &a, nil
}

func (r *Repo) GetBooks() ([]string, error) {
	QUERY := `SELECT * FROM books WHERE deleted = 0`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query books: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get book columns: %w", err)
	}

	books := make([]string, 0)

	for rows.Next() {
		columns := make([]sql.NullString, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, fmt.Errorf("scan book: %w", err)
		}
		var sb strings.Builder
		for i := range cols {
			fmt.Fprintf(&sb, "%s, ", columns[i].String)
		}

		books = append(books, sb.String())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books: %w", err)
	}

	return books, nil
}

func (r *Repo) GetBooksByLetter(letters string) ([]book.Book, error) {
	pattern := cases.Title(language.Und, cases.NoLower).String(letters) + "%"
	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE b.title LIKE ? COLLATE NOCASE AND b.deleted = 0
		ORDER BY b.title
	`

	rows, err := r.db.Query(QUERY, pattern)
	if err != nil {
		return nil, fmt.Errorf("query books by letter: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, fmt.Errorf("scan book by letter: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		// Helper to construct series info
		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			// Check if author is new
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}

			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}

			// If series is missing, add it
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}
		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by letter: %w", err)
	}

	// Re-sort in Go to be fast and correct (since map randomization)
	books := make([]book.Book, 0, len(booksMap))
	for _, book := range booksMap {
		books = append(books, *book)
	}

	sortBooks(books)

	return books, nil
}

// sortBooks sorts books by Author -> Series -> SeriesNo -> Title
func sortBooks(books []book.Book) {
	sort.Slice(books, func(i, j int) bool {
		b1 := &books[i]
		b2 := &books[j]

		// 1. Author (Last Name, First Name) - First author only
		var a1, a2 book.Author
		if len(b1.Author) > 0 {
			a1 = b1.Author[0]
		}
		if len(b2.Author) > 0 {
			a2 = b2.Author[0]
		}

		if a1.LastName != a2.LastName {
			return strings.ToLower(a1.LastName) < strings.ToLower(a2.LastName)
		}
		if a1.FirstName != a2.FirstName {
			return strings.ToLower(a1.FirstName) < strings.ToLower(a2.FirstName)
		}

		// 2. Series Name
		var s1, s2 string
		if b1.Series != nil {
			s1 = b1.Series.Name
		}
		if b2.Series != nil {
			s2 = b2.Series.Name
		}
		if s1 != s2 {
			// Books with series usually come together.
			// If one has series and other doesn't (and authors match),
			// usually we want to group series.
			// Plain string comparison works: empty string < non-empty.
			// So books without series come first? Or last?
			// Let's stick to standard string compare for now.
			return strings.ToLower(s1) < strings.ToLower(s2)
		}

		// 3. Series Number
		var sn1, sn2 int
		if b1.Series != nil {
			sn1 = b1.Series.SeriesNo
		}
		if b2.Series != nil {
			sn2 = b2.Series.SeriesNo
		}
		if sn1 != sn2 {
			return sn1 < sn2
		}

		// 4. Title
		return strings.ToLower(b1.Title) < strings.ToLower(b2.Title)
	})
}

func (r *Repo) GetBooksByAuthorID(id int64) ([]book.Book, error) {
	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE ba.author_id = ? AND b.deleted = 0
		ORDER BY b.title
	`

	rows, err := r.db.Query(QUERY, id)
	if err != nil {
		return nil, fmt.Errorf("query books by author id: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, fmt.Errorf("scan book by author id: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		// Helper to construct series info
		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			// Check if author is new (avoid duplicates if multiple series rows caused duplication)
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}

			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}

			// If series is missing, add it
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}

		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by author id: %w", err)
	}

	// Re-sort using universal logic
	books := make([]book.Book, 0, len(booksMap))
	for _, book := range booksMap {
		books = append(books, *book)
	}

	sortBooks(books)

	return books, nil
}

func (r *Repo) GetGenres() ([]book.Genre, error) {
	QUERY := `
		SELECT g.genre_id, g.name, g.display_name
		FROM genres g
		JOIN book_genres bg ON g.genre_id = bg.genre_id
		JOIN books b ON bg.book_id = b.book_id
		WHERE b.deleted = 0
		GROUP BY g.genre_id
		ORDER BY g.display_name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query genres: %w", err)
	}
	defer rows.Close()

	genres := make([]book.Genre, 0)
	for rows.Next() {
		var g book.Genre
		var displayName sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &displayName); err != nil {
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		if displayName.Valid {
			g.DisplayName = displayName.String
		} else {
			g.DisplayName = g.Name // Fallback
		}
		genres = append(genres, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate genres: %w", err)
	}

	return genres, nil
}

func (r *Repo) GetBookByID(id int64) (*book.Book, error) {
	QUERY := `
		SELECT book_id, title, lang, archive, filename,
			   file_size, date_added, lib_id, deleted, lib_rate
		FROM books
		WHERE book_id = ? AND deleted = 0
	`

	var b book.Book
	var deleted bool
	var libRate sql.NullInt64

	err := r.db.QueryRow(QUERY, id).Scan(
		&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
		&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get book by ID %d: %w", id, err)
	}

	b.Deleted = deleted
	if libRate.Valid {
		b.LibRate = int(libRate.Int64)
	}

	// Fetch related data
	if err := r.fetchBookDetails(&b); err != nil {
		return nil, err
	}

	return &b, nil
}

// GetRecentBooks returns recently added books with pagination
func (r *Repo) GetRecentBooks(limit, offset int) ([]book.Book, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM books WHERE deleted = 0`
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count recent books: %w", err)
	}

	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE b.deleted = 0
		ORDER BY b.date_added DESC, b.book_id DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(QUERY, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query recent books: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	bookOrder := make([]int64, 0)

	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, 0, fmt.Errorf("scan recent book: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}
			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}
		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
			bookOrder = append(bookOrder, b.BookID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate recent books: %w", err)
	}

	// Preserve order (most recent first)
	books := make([]book.Book, 0, len(bookOrder))
	for _, id := range bookOrder {
		books = append(books, *booksMap[id])
	}

	return books, total, nil
}

// GetBooksByGenre returns books by genre with pagination
func (r *Repo) GetBooksByGenre(genre string, limit, offset int) ([]book.Book, int, error) {
	// Get total count for this genre
	countQuery := `
		SELECT COUNT(DISTINCT b.book_id)
		FROM books b
		JOIN book_genres bg ON b.book_id = bg.book_id
		JOIN genres g ON bg.genre_id = g.genre_id
		WHERE (g.display_name = ? OR g.name = ?) AND b.deleted = 0
	`
	var total int
	if err := r.db.QueryRow(countQuery, genre, genre).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count books by genre: %w", err)
	}

	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate,
			   a.first_name, a.middle_name, a.last_name,
			   s.series_id, s.name, bs.series_no
		FROM books b
		JOIN book_genres bg ON b.book_id = bg.book_id
		JOIN genres g ON bg.genre_id = g.genre_id
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		WHERE (g.display_name = ? OR g.name = ?) AND b.deleted = 0
		ORDER BY b.title
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(QUERY, genre, genre, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query books by genre: %w", err)
	}
	defer rows.Close()

	booksMap := make(map[int64]*book.Book)
	bookOrder := make([]int64, 0)

	for rows.Next() {
		var b book.Book
		var author book.Author
		var firstName, middleName, lastName sql.NullString
		var deleted bool
		var libRate sql.NullInt64
		var seriesID sql.NullInt64
		var seriesName sql.NullString
		var seriesNo sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
			&firstName, &middleName, &lastName,
			&seriesID, &seriesName, &seriesNo,
		); err != nil {
			return nil, 0, fmt.Errorf("scan book by genre: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		var seriesInfo *book.SeriesInfo
		if seriesName.Valid {
			seriesInfo = &book.SeriesInfo{
				ID:       seriesID.Int64,
				Name:     seriesName.String,
				SeriesNo: int(seriesNo.Int64),
			}
		}

		if existingBook, ok := booksMap[b.BookID]; ok {
			isNewAuthor := true
			for _, a := range existingBook.Author {
				if a.FirstName == firstName.String && a.LastName == lastName.String {
					isNewAuthor = false
					break
				}
			}
			if isNewAuthor && (firstName.Valid || middleName.Valid || lastName.Valid) {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				existingBook.Author = append(existingBook.Author, author)
			}
			if existingBook.Series == nil && seriesInfo != nil {
				existingBook.Series = seriesInfo
			}
		} else {
			if firstName.Valid || middleName.Valid || lastName.Valid {
				author.FirstName = firstName.String
				author.MiddleName = middleName.String
				author.LastName = lastName.String
				b.Author = []book.Author{author}
			}
			if seriesInfo != nil {
				b.Series = seriesInfo
			}
			booksMap[b.BookID] = &b
			bookOrder = append(bookOrder, b.BookID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate books by genre: %w", err)
	}

	// Preserve order
	books := make([]book.Book, 0, len(bookOrder))
	for _, id := range bookOrder {
		books = append(books, *booksMap[id])
	}

	sortBooks(books)

	return books, total, nil
}

// NEW: Fetch all related details for a book
func (r *Repo) fetchBookDetails(b *book.Book) error {
	// Fetch authors
	authorsQuery := `
		SELECT a.author_id, a.first_name, a.middle_name, a.last_name
		FROM authors a
		JOIN book_authors ba ON a.author_id = ba.author_id
		WHERE ba.book_id = ?
		ORDER BY a.last_name, a.first_name
	`

	rows, err := r.db.Query(authorsQuery, b.BookID)
	if err != nil {
		return fmt.Errorf("query authors for book %d: %w", b.BookID, err)
	}
	defer rows.Close()

	authors := make([]book.Author, 0)
	for rows.Next() {
		var a book.Author
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName); err != nil {
			return fmt.Errorf("scan author for book %d: %w", b.BookID, err)
		}
		authors = append(authors, a)
	}
	b.Author = authors

	seriesQuery := `
		SELECT s.series_id, s.name, bs.series_no
		FROM series s
		JOIN book_series bs ON s.series_id = bs.series_id
		WHERE bs.book_id = ?
	`

	var seriesID int64
	var seriesName sql.NullString
	var seriesNo sql.NullInt64

	err = r.db.QueryRow(seriesQuery, b.BookID).Scan(&seriesID, &seriesName, &seriesNo)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("query series for book %d: %w", b.BookID, err)
	}

	if seriesName.Valid {
		b.Series = &book.SeriesInfo{
			ID:       seriesID,
			Name:     seriesName.String,
			SeriesNo: int(seriesNo.Int64),
		}
	}

	keywordsQuery := `
		SELECT k.keyword_id, k.name
		FROM keywords k
		JOIN book_keywords bk ON k.keyword_id = bk.keyword_id
		WHERE bk.book_id = ?
		ORDER BY k.name
	`

	rows, err = r.db.Query(keywordsQuery, b.BookID)
	if err != nil {
		return fmt.Errorf("query keywords for book %d: %w", b.BookID, err)
	}
	defer rows.Close()

	keywords := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(new(int64), &name); err != nil {
			return err
		}
		keywords = append(keywords, name)
	}
	b.Keywords = keywords

	return nil
}

// GetSeries Get all series
func (r *Repo) GetSeries() ([]book.SeriesInfo, error) {
	QUERY := `
		SELECT DISTINCT s.series_id, s.name
		FROM series s
		JOIN book_series bs ON s.series_id = bs.series_id
		JOIN books b ON bs.book_id = b.book_id
		WHERE b.deleted = 0
		ORDER BY s.name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query series: %w", err)
	}
	defer rows.Close()

	series := make([]book.SeriesInfo, 0)
	for rows.Next() {
		var s book.SeriesInfo
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, fmt.Errorf("scan series: %w", err)
		}
		series = append(series, s)
	}

	return series, nil
}

// GetBooksBySeriesID books by series
func (r *Repo) GetBooksBySeriesID(seriesID int64) ([]book.Book, error) {
	QUERY := `
		SELECT b.book_id, b.title, b.lang, b.archive, b.filename,
			   b.file_size, b.date_added, b.lib_id, b.deleted, b.lib_rate
		FROM books b
		JOIN book_series bs ON b.book_id = bs.book_id
		WHERE bs.series_id = ? AND b.deleted = 0
		ORDER BY bs.series_no, b.title
	`

	rows, err := r.db.Query(QUERY, seriesID)
	if err != nil {
		return nil, fmt.Errorf("query books by series: %w", err)
	}
	defer rows.Close()

	books := make([]book.Book, 0)
	for rows.Next() {
		var b book.Book
		var deleted bool
		var libRate sql.NullInt64

		if err := rows.Scan(
			&b.BookID, &b.Title, &b.Lang, &b.Archive, &b.FileName,
			&b.FileSize, &b.DateAdded, &b.LibID, &deleted, &libRate,
		); err != nil {
			return nil, fmt.Errorf("scan book: %w", err)
		}

		b.Deleted = deleted
		if libRate.Valid {
			b.LibRate = int(libRate.Int64)
		}

		books = append(books, b)
	}

	sortBooks(books)

	return books, nil
}

// GetKeywords Get all keywords
func (r *Repo) GetKeywords() ([]book.Keyword, error) {
	QUERY := `
		SELECT DISTINCT k.keyword_id, k.name
		FROM keywords k
		JOIN book_keywords bk ON k.keyword_id = bk.keyword_id
		JOIN books b ON bk.book_id = b.book_id
		WHERE b.deleted = 0
		ORDER BY k.name
	`

	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("query keywords: %w", err)
	}
	defer rows.Close()

	keywords := make([]book.Keyword, 0)
	for rows.Next() {
		var k book.Keyword
		if err := rows.Scan(&k.ID, &k.Name); err != nil {
			return nil, fmt.Errorf("scan keyword: %w", err)
		}
		keywords = append(keywords, k)
	}

	return keywords, nil
}

// SearchBooks performs full-text search across book titles and authors
// Uses FTS5 for fast, ranked search results
// Optimized with single query including author JOIN (fixes N+1 query issue)
func (r *Repo) SearchBooks(ctx context.Context, query string, limit, offset int, fields []string, languages []string) ([]book.BookSearchResult, error) {
	// Validate query
	if query == "" {
		return []book.BookSearchResult{}, nil
	}

	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return []book.BookSearchResult{}, nil
	}

	// Escape FTS5 special characters to prevent injection
	escapedQuery := escapeFTS5Query(cleanQuery)
	var ftsQuery string

	// If specific fields are requested, restrict the search
	if len(fields) > 0 {
		var parts []string
		for _, f := range fields {
			parts = append(parts, fmt.Sprintf("%s:%s*", f, escapedQuery))
		}
		// Combine with OR: (title:foo* OR author:foo*)
		// We use standard FTS5 syntax
		ftsQuery = fmt.Sprintf("(%s)", strings.Join(parts, " OR "))
	} else {
		// Default search across all columns
		ftsQuery = escapedQuery + "*"
	}

	// Base arguments required for building the query b.c. sql doesn't support slice arguments as IN clause
	var args []interface{}
	args = append(args, ftsQuery)

	// Search FTS5 table and join back to books table for full details
	// Uses book_id column for direct, accurate mapping
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT
			b.book_id,
			b.title,
			b.lang,
			b.archive,
			b.filename,
			b.file_size,
			b.deleted,
			s.name as series_name,
			bs.series_no,
			fts.rank,
			group_concat(distinct a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, '')) as author,
			group_concat(distinct g.display_name) as genres
		FROM books_fts fts
		JOIN books b ON fts.book_id = b.book_id
		LEFT JOIN book_authors ba ON b.book_id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.author_id
		LEFT JOIN book_series bs ON b.book_id = bs.book_id
		LEFT JOIN series s ON bs.series_id = s.series_id
		LEFT JOIN book_genres bg ON b.book_id = bg.book_id
		LEFT JOIN genres g ON bg.genre_id = g.genre_id
		WHERE books_fts MATCH ? AND b.deleted = 0 `)

	// Language filter condition
	langCondition := ""
	if len(languages) > 0 {
		// Treat empty language as "ru"
		// If "ru" is requested, also include "" (empty string) in the loop/IN clause
		searchLangs := make([]string, 0, len(languages)+1)
		for _, l := range languages {
			searchLangs = append(searchLangs, l)
			if strings.EqualFold(l, "ru") {
				searchLangs = append(searchLangs, "")
			}
		}

		lArgs, placeholders := buildSliceArgs(searchLangs)
		langCondition = fmt.Sprintf("AND b.lang IN (%s)", placeholders)
		args = append(args, lArgs...)
	}
	queryBuilder.WriteString(" ")
	queryBuilder.WriteString(langCondition)

	queryBuilder.WriteString(`
		GROUP BY b.book_id, b.title, b.lang, b.archive, b.filename, b.file_size, b.deleted, s.name, bs.series_no, fts.rank
		ORDER BY author, s.name, bs.series_no, b.title COLLATE NOCASE
		LIMIT ? OFFSET ?
	`)

	QUERY := queryBuilder.String()
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, QUERY, args...)
	if err != nil {
		return nil, fmt.Errorf("search books: %w", err)
	}
	defer rows.Close()

	results := make([]book.BookSearchResult, 0)
	for rows.Next() {
		var r book.BookSearchResult
		var seriesName sql.NullString
		var seriesNo sql.NullInt64
		var genresStr sql.NullString
		var authorStr sql.NullString

		err := rows.Scan(
			&r.BookID, &r.Title, &r.Lang, &r.Archive, &r.FileName,
			&r.FileSize, &r.Deleted, &seriesName, &seriesNo,
			&r.Rank, &authorStr, &genresStr,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}

		if authorStr.Valid {
			r.Author = authorStr.String
		}
		if seriesName.Valid {
			r.SeriesName = seriesName.String
		}
		if seriesNo.Valid {
			r.SeriesNo = int(seriesNo.Int64)
		}
		if genresStr.Valid {
			r.Genres = strings.Split(genresStr.String, ",")
		}

		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	return results, nil
}

// RebuildFTSIndex rebuilds the full-text search index for all books
// Updates the author field in books_fts to include properly concatenated author names
func (r *Repo) RebuildFTSIndex() error {
	// Rebuild FTS index from scratch
	// 1. Clear FTS table
	if _, err := r.db.Exec("DELETE FROM books_fts"); err != nil {
		return fmt.Errorf("rebuild FTS index (delete): %w", err)
	}

	// 2. Insert all books with their metadata
	QUERY := `
		INSERT INTO books_fts(title, author, series, genre, book_id)
		SELECT
			b.title,
			(SELECT group_concat(a.last_name || ' ' || a.first_name || ' ' || coalesce(a.middle_name, ''), ' | ')
			 FROM book_authors ba
			 JOIN authors a ON ba.author_id = a.author_id
			 WHERE ba.book_id = b.book_id),
			(SELECT s.name
			 FROM book_series bs
			 JOIN series s ON bs.series_id = s.series_id
			 WHERE bs.book_id = b.book_id),
			(SELECT group_concat(g.name || ' ' || coalesce(g.display_name, '') || ' ' || coalesce(g.translit_name, ''), ' | ')
			 FROM book_genres bg
			 JOIN genres g ON bg.genre_id = g.genre_id
			 WHERE bg.book_id = b.book_id),
			b.book_id
		FROM books b
		WHERE b.deleted = 0
	`
	result, err := r.db.Exec(QUERY)
	if err != nil {
		return fmt.Errorf("rebuild FTS index (insert): %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Info("FTS index rebuilt", "rows_updated", rowsAffected)

	return nil
}

func (r *Repo) migrateAddTranslitName() {
	_, err := r.db.Exec("ALTER TABLE genres ADD COLUMN translit_name TEXT")
	if err != nil {
		// Ignore error if column already exists
		// SQLite doesn't have "ADD COLUMN IF NOT EXISTS"
		if !strings.Contains(err.Error(), "duplicate column name") {
			logger.Warn("Failed to add translit_name column (might already exist)", "error", err)
		}
	}
}

func (r *Repo) SyncGenreDisplayNames() {
	rows, err := r.db.Query(`SELECT name, display_name, translit_name FROM genres`)
	if err != nil {
		logger.Error("Failed to query genres for update", "error", err)
		return
	}

	type genreUpdate struct {
		name         string
		displayName  string
		translitName string
	}
	var updates []genreUpdate

	for rows.Next() {
		var name string
		var currentDN sql.NullString
		var currentTN sql.NullString
		if err := rows.Scan(&name, &currentDN, &currentTN); err == nil {
			displayName := MapGenre(name)
			translitName := Translit(displayName)

			needsUpdate := false
			if displayName != name && (!currentDN.Valid || currentDN.String != displayName) {
				needsUpdate = true
			}
			if !currentTN.Valid || currentTN.String != translitName {
				needsUpdate = true
			}

			if needsUpdate {
				updates = append(updates, genreUpdate{name, displayName, translitName})
			}
		}
	}
	rows.Close()

	if len(updates) == 0 {
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		logger.Error("Failed to begin transaction for genre update", "error", err)
		return
	}

	stmt, err := tx.Prepare(`UPDATE genres SET display_name = ?, translit_name = ? WHERE name = ?`)
	if err != nil {
		logger.Error("Failed to prepare statement for genre update", "error", err)
		if err := tx.Rollback(); err != nil {
			logger.Error("Failed to rollback transaction", "error", err)
		}
		return
	}
	defer stmt.Close()

	count := 0
	for _, u := range updates {
		if _, err := stmt.Exec(u.displayName, u.translitName, u.name); err != nil {
			logger.Error("Failed to execute genre update", "error", err, "name", u.name)
		} else {
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Error("Failed to commit genre updates", "error", err)
	} else if count > 0 {
		logger.Info("Updated genre display/translit names", "new_count", count)
	}
}

// GetLanguages returns a list of distinct languages from non-deleted books
func (r *Repo) GetLanguages() ([]string, error) {
	// Treat empty or null language as 'ru'
	QUERY := `
		SELECT DISTINCT 
			CASE WHEN IFNULL(lang, '') = '' THEN 'ru' ELSE lang END as language 
		FROM books 
		WHERE deleted = 0 
		ORDER BY language
	`
	rows, err := r.db.Query(QUERY)
	if err != nil {
		return nil, fmt.Errorf("get languages: %w", err)
	}
	defer rows.Close()

	var languages []string
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			return nil, fmt.Errorf("scan language: %w", err)
		}
		languages = append(languages, lang)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate languages: %w", err)
	}
	return languages, nil
}

// buildSliceArgs generates placeholders and converts slice to []interface{}
// e.g. buildSliceArgs([]string{"a", "b"}) -> ([]interface{}{"a", "b"}, "?,?")
func buildSliceArgs(items []string) ([]interface{}, string) {
	if len(items) == 0 {
		return nil, ""
	}
	args := make([]interface{}, len(items))
	placeholders := make([]string, len(items))
	for i, item := range items {
		args[i] = item
		placeholders[i] = "?"
	}
	return args, strings.Join(placeholders, ",")
}
