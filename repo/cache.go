package repo

import (
	"fmt"

	"github.com/htol/bopds/logger"
)

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

	return nil
}
