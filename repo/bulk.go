package repo

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
)

type linkData struct {
	bookID  int64
	otherID int64
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
	var bookAuthors []linkData
	var bookGenres []linkData
	var bookKeywords []linkData

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

		// Resolve Genres
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
