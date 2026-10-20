package github

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"lazydev/internal/state"
)

// Cache ist der letzte erfolgreich geladene Stand eines Projekts.
type Cache struct {
	Repo      string    `json:"repo"`
	FetchedAt time.Time `json:"fetchedAt"`
	Issues    []Issue   `json:"issues"`
}

// LoadCache liefert nil, nil, wenn es keinen Zwischenspeicher gibt.
func LoadCache(projectID string) (*Cache, error) {
	data, err := os.ReadFile(state.IssuesPath(projectID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := &Cache{}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// SaveCache schreibt atomar, damit ein Neustart nie eine halbe Datei liest.
func SaveCache(projectID string, c *Cache) error {
	path := state.IssuesPath(projectID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".issues-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
