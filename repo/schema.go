package repo

import (
	"database/sql"
	"time"

	"github.com/htol/bopds/config"
	"github.com/htol/bopds/logger"
	_ "github.com/mattn/go-sqlite3"
)

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

// migrateAddTranslitName checks if 'translit_name' column exists in 'genres' table, and adds it if missing
func (r *Repo) migrateAddTranslitName() {
	// Check if column exists
	// PRAGMA has no result if column missing? No, table_info returns all columns.
	// We can try to add it and ignore error if it exists?
	// SQLite 'ADD COLUMN' will fail if it exists.

	// Simple check: SELECT translit_name FROM genres LIMIT 1
	rows, err := r.db.Query("SELECT translit_name FROM genres LIMIT 1")
	if err == nil {
		rows.Close()
		return // Column exists
	}

	logger.Info("Migrating database: adding 'translit_name' to 'genres' table")
	_, err = r.db.Exec("ALTER TABLE genres ADD COLUMN translit_name TEXT")
	if err != nil {
		logger.Error("Failed to add 'translit_name' column", "error", err)
	}
}
