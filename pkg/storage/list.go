package storage

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListKeys returns up to limit secret names for dashboard rendering.
func (s *Store) ListKeys(limit int) ([]string, error) {
	if s.Sealed() {
		return nil, ErrSealed
	}
	secretsDir := filepath.Join(s.root, "secrets")
	entries, err := os.ReadDir(secretsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".vlx") {
			names = append(names, strings.TrimSuffix(name, ".vlx"))
		}
	}
	sort.Strings(names)
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}
	return names, nil
}
