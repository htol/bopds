package scanner

import (
	"fmt"
	"os"
	"path/filepath"
)

// Library is a first-level subdirectory of a library root.
type Library struct {
	Name string // basename of the library directory
	Root string // root the library was discovered under
	Path string // full path to the library directory
}

// DiscoverLibraries returns the first-level subdirectories of each root as
// libraries, in root order. A duplicate library name across roots is an error.
func DiscoverLibraries(roots []string) ([]Library, error) {
	seen := make(map[string]string) // library name -> path of first occurrence
	var libraries []Library

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("read library root %s: %w", root, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			path := filepath.Join(root, entry.Name())
			if first, dup := seen[entry.Name()]; dup {
				return nil, fmt.Errorf("duplicate library name %q: %s and %s", entry.Name(), first, path)
			}
			seen[entry.Name()] = path

			libraries = append(libraries, Library{
				Name: entry.Name(),
				Root: root,
				Path: path,
			})
		}
	}

	return libraries, nil
}
