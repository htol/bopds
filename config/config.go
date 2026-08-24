package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Library  LibraryConfig
	LogLevel string
}

type ServerConfig struct {
	Port         int
	URLPrefix    string
	ReadTimeout  int // seconds
	WriteTimeout int // seconds
	IdleTimeout  int // seconds
}

type DatabaseConfig struct {
	Path            string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // seconds
	BatchSize       int
}

type LibraryConfig struct {
	Paths               []string
	Names               map[string]string
	MissingRowThreshold int // percent of rows a rescan may tombstone, 100 disables the guard
}

// Load creates a new Config from environment variables with defaults
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnvInt("PORT", 3001),
			URLPrefix:    getEnv("URL_PREFIX", ""),
			ReadTimeout:  getEnvInt("READ_TIMEOUT", 15),
			WriteTimeout: getEnvInt("WRITE_TIMEOUT", 15),
			IdleTimeout:  getEnvInt("IDLE_TIMEOUT", 60),
		},
		Database: DatabaseConfig{
			Path:            getEnv("DB_PATH", "books.db"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 25),
			ConnMaxLifetime: getEnvInt("DB_CONN_MAX_LIFETIME", 300),
			BatchSize:       getEnvInt("DB_BATCH_SIZE", 1000),
		},
		Library: LibraryConfig{
			Paths: splitPaths(getEnv("LIBRARY_PATH", "./lib")),
			Names:               parseLibraryNames(getEnv("LIBRARY_NAMES", "")),
			MissingRowThreshold: getEnvInt("LIBRARY_MISSING_THRESHOLD", 30),
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// parseLibraryNames parses "slug:Name,..." into a slug-to-display-name map.
// Entries without a colon are skipped with a warning.
func parseLibraryNames(value string) map[string]string {
	names := make(map[string]string)
	if value == "" {
		return names
	}
	for _, entry := range strings.Split(value, ",") {
		slug, name, found := strings.Cut(entry, ":")
		if !found {
			slog.Warn("Skipping malformed LIBRARY_NAMES entry", "entry", entry)
			continue
		}
		names[slug] = name
	}
	return names
}

// splitPaths splits a PATH-style list on ":", dropping empty segments.
func splitPaths(value string) []string {
	var paths []string
	for _, p := range strings.Split(value, ":") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
