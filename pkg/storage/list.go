package storage

import (
	"os"
	"path/filepath"
	"strings"
)

// SpaceSummary contains aggregated info for templates/dashboard.
type SpaceSummary struct {
	Space      string
	Namespaces []NamespaceSummary
}

// NamespaceSummary lists secret names (partial) within a namespace.
type NamespaceSummary struct {
	Namespace string
	Secrets   []string
}

// ListSpaces returns the spaces/namespaces and secret names under the store root.
func (s *Store) ListSpaces(limit int) ([]SpaceSummary, error) {
	if s.Sealed() {
		return nil, ErrSealed
	}
	spacesDir := filepath.Join(s.root, "spaces")
	entries, err := os.ReadDir(spacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var summaries []SpaceSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		space := entry.Name()
		nsEntries, err := os.ReadDir(filepath.Join(spacesDir, space))
		if err != nil {
			continue
		}
		spaceSummary := SpaceSummary{Space: space}
		for _, nsEntry := range nsEntries {
			if !nsEntry.IsDir() {
				continue
			}
			namespace := nsEntry.Name()
			secretFiles, err := os.ReadDir(filepath.Join(spacesDir, space, namespace))
			if err != nil {
				continue
			}
			var secrets []string
			for _, secretFile := range secretFiles {
				name := secretFile.Name()
				if strings.HasSuffix(name, ".vlx") {
					secrets = append(secrets, strings.TrimSuffix(name, ".vlx"))
				}
			}
			if limit > 0 && len(secrets) > limit {
				secrets = secrets[:limit]
			}
			spaceSummary.Namespaces = append(spaceSummary.Namespaces, NamespaceSummary{Namespace: namespace, Secrets: secrets})
		}
		summaries = append(summaries, spaceSummary)
	}
	return summaries, nil
}
