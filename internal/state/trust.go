package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"lazydev/internal/fsutil"
)

// TrustPath holds the trusted hash over allowedTools and hooks of a project's config.yaml.
func TrustPath(id string) string {
	return filepath.Join(Dir(), "trust", id+".json")
}

// Trusted reports whether hash is trusted for project id. An empty hash is always trusted:
// without hooks and allowed tools there is nothing to confirm.
func Trusted(id, hash string) bool {
	if hash == "" {
		return true
	}
	data, err := os.ReadFile(TrustPath(id))
	if err != nil {
		return false
	}
	var f struct {
		Hash string `json:"hash"`
	}
	return json.Unmarshal(data, &f) == nil && f.Hash == hash
}

// Trust saves hash as trusted for project id.
func Trust(id, hash string) error {
	data, _ := json.MarshalIndent(map[string]string{"hash": hash}, "", "  ")
	return fsutil.WriteAtomic(TrustPath(id), append(data, '\n'), 0o644)
}
