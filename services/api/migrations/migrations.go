package migrations

import (
	"embed"
	"sort"
)

//go:embed *.sql
var FS embed.FS

// List returns embedded migration filenames in sorted order.
func List() ([]string, error) {
	entries, err := FS.ReadDir(".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// Read returns the content of an embedded migration file.
func Read(name string) ([]byte, error) {
	return FS.ReadFile(name)
}
