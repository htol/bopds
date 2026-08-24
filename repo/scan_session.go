package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/htol/bopds/book"
	"github.com/htol/bopds/logger"
)

// LibraryScanSession tracks the books seen during one scan of a library.
// On Finish, rows of the library that were not seen are tombstoned
// (deleted = 1) in a single transaction; a later scan that sees a row again
// clears the flag (via the AddBatch upsert).
type LibraryScanSession struct {
	repo      *Repo
	libraryID int64

	mu        sync.Mutex
	seenLibID map[int64]struct{}
	seenFile  map[string]struct{}
}

// BeginLibraryScan starts a scan session for the library. While active, every
// AddBatch records the natural keys it upserts for this library; Finish
// tombstones the rows absent from that snapshot.
func (r *Repo) BeginLibraryScan(libraryID int64) *LibraryScanSession {
	s := &LibraryScanSession{
		repo:      r,
		libraryID: libraryID,
		seenLibID: make(map[int64]struct{}),
		seenFile:  make(map[string]struct{}),
	}
	r.mu.Lock()
	if r.sessions == nil {
		r.sessions = make(map[int64]*LibraryScanSession)
	}
	r.sessions[libraryID] = s
	r.mu.Unlock()
	return s
}

// record marks the natural key of one upserted book as seen.
func (s *LibraryScanSession) record(b *book.Book) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.LibID != 0 {
		s.seenLibID[b.LibID] = struct{}{}
	} else {
		s.seenFile[fileKey(b.Archive, b.FileName)] = struct{}{}
	}
}

// recordScanned marks records' natural keys as seen in any active session
// for their library.
func (r *Repo) recordScanned(records []*book.Book) {
	for _, b := range records {
		libID, err := r.cachedLibraryID(b.Library)
		if err != nil {
			continue
		}
		r.mu.RLock()
		s := r.sessions[libID]
		r.mu.RUnlock()
		if s != nil {
			s.record(b)
		}
	}
}

// notSeenPredicate matches library rows whose natural key was not seen during
// the session. Used as a static SQL fragment with bound parameters.
const notSeenPredicate = `NOT (
			(lib_id <> 0 AND lib_id IN (SELECT lib_id FROM seen_libids))
			OR
			(lib_id = 0 AND (archive, filename) IN (SELECT archive, filename FROM seen_files))
		)`

// Finish tombstones the library rows not seen during the scan, in one
// transaction, and unregisters the session.
func (s *LibraryScanSession) Finish() error {
	// Unregister first so concurrent AddBatches cannot extend the snapshot
	s.repo.mu.Lock()
	if s.repo.sessions[s.libraryID] == s {
		delete(s.repo.sessions, s.libraryID)
	}
	s.repo.mu.Unlock()

	s.mu.Lock()
	seenLibID := s.seenLibID
	seenFile := s.seenFile
	s.mu.Unlock()

	r := s.repo
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tombstone transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logger.Error("Failed to rollback tombstone transaction", "error", err)
		}
	}()

	// Temp tables are scoped to this transaction's connection
	if _, err = tx.Exec(`DROP TABLE IF EXISTS temp.seen_libids`); err != nil {
		return err
	}
	if _, err = tx.Exec(`CREATE TEMP TABLE seen_libids(lib_id INTEGER PRIMARY KEY)`); err != nil {
		return err
	}
	if _, err = tx.Exec(`DROP TABLE IF EXISTS temp.seen_files`); err != nil {
		return err
	}
	if _, err = tx.Exec(`CREATE TEMP TABLE seen_files(archive TEXT NOT NULL, filename TEXT NOT NULL, PRIMARY KEY (archive, filename))`); err != nil {
		return err
	}

	if err = insertSeenKeys(tx, seenLibID, seenFile); err != nil {
		return err
	}

	result, err := tx.Exec(`
		UPDATE books SET deleted = 1
		WHERE library_id = ? AND deleted = 0 AND `+notSeenPredicate, s.libraryID)
	if err != nil {
		return fmt.Errorf("tombstone missing books: %w", err)
	}
	tombstoned, _ := result.RowsAffected()

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tombstone transaction: %w", err)
	}

	logger.Info("Library scan finished", "library_id", s.libraryID, "tombstoned", tombstoned)
	return nil
}

// insertSeenKeys bulk-inserts the seen natural keys into the temp tables.
func insertSeenKeys(tx *sql.Tx, seenLibID map[int64]struct{}, seenFile map[string]struct{}) error {
	libIDs := make([]interface{}, 0, len(seenLibID))
	for id := range seenLibID {
		libIDs = append(libIDs, id)
	}
	const chunk = 900
	for i := 0; i < len(libIDs); i += chunk {
		end := i + chunk
		if end > len(libIDs) {
			end = len(libIDs)
		}
		ph := strings.TrimSuffix(strings.Repeat("(?),", end-i), ",")
		if _, err := tx.Exec(`INSERT OR IGNORE INTO seen_libids(lib_id) VALUES `+ph, libIDs[i:end]...); err != nil {
			return err
		}
	}

	files := make([]interface{}, 0, len(seenFile)*2)
	for key := range seenFile {
		archive, filename, _ := strings.Cut(key, "\x00")
		files = append(files, archive, filename)
	}
	const chunkFiles = 450 // 2 params per row
	for i := 0; i < len(files); i += chunkFiles * 2 {
		end := i + chunkFiles*2
		if end > len(files) {
			end = len(files)
		}
		rows := (end - i) / 2
		ph := strings.TrimSuffix(strings.Repeat("(?, ?),", rows), ",")
		if _, err := tx.Exec(`INSERT OR IGNORE INTO seen_files(archive, filename) VALUES `+ph, files[i:end]...); err != nil {
			return err
		}
	}
	return nil
}
