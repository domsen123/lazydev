package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// trustFile hält den bestätigten Hash über allowedTools und hooks aus .lazydev/config.yaml.
type trustFile struct {
	Hash string `json:"hash"`
}

// TrustPath ist die Bestätigung der Hooks und Freigaben eines Projekts.
func TrustPath(projectID string) string {
	return filepath.Join(Dir(), "trust", projectID+".json")
}

// Trusted meldet, ob hash für das Projekt bestätigt ist. Ein leerer Hash gilt immer als vertraut:
// Ohne Hooks und Freigaben ist nichts zu bestätigen.
func Trusted(projectID, hash string) bool {
	if hash == "" {
		return true
	}
	data, err := os.ReadFile(TrustPath(projectID))
	if err != nil {
		return false
	}
	var f trustFile
	return json.Unmarshal(data, &f) == nil && f.Hash == hash
}

// Trust speichert hash als bestätigt für das Projekt.
func Trust(projectID, hash string) error {
	path := TrustPath(projectID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(trustFile{Hash: hash}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".trust-*.json")
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
