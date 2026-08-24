package repo

import (
	"database/sql"
	"sync"

	"github.com/htol/bopds/logger"
)

type Repo struct {
	db   *sql.DB
	path string

	mu                  sync.RWMutex
	authorCache         map[string]int64
	genreCache          map[string]int64
	seriesCache         map[string]int64
	keywordCache        map[string]int64
	libraryCache        map[string]int64
	sessions            map[int64]*LibraryScanSession
	missingRowThreshold int // percent of a library's rows a scan may tombstone; 100 disables the guard
}

func (r *Repo) Close() error {
	if r.db != nil {
		logger.Info("Closing database connection")
		return r.db.Close()
	}
	return nil
}

func (r *Repo) Ping() error {
	if r.db != nil {
		return r.db.Ping()
	}
	return sql.ErrConnDone
}

// List placeholder to satisfy interface
func (r *Repo) List() error {
	return nil
}

// Search placeholder to satisfy interface
func (r *Repo) Search() error {
	return nil
}
