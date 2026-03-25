package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ListKeys returns up to limit secret names for dashboard rendering.
func (s *Store) ListKeys(limit int) ([]SecretSummary, error) {
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
	var summaries []SecretSummary
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".vlx") {
			continue
		}
		secretName := strings.TrimSuffix(name, ".vlx")
		summary := SecretSummary{Name: secretName}
		if info, err := os.Stat(filepath.Join(secretsDir, name)); err == nil {
			summary.UpdatedAt = info.ModTime()
		}
		if env, err := readEnvelope(filepath.Join(secretsDir, name)); err == nil {
			summary.Version = env.Version
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })
	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}

type SecretSummary struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

func readEnvelope(path string) (*secretEnvelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env secretEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
