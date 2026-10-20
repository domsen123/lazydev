// Package chatrun führt die Chats mit /create-prd als losgelöste Runden aus. Jede Runde läuft als
// eigener Prozess "lazydev chat <projekt-id> <chat-id>". Zustand und Log liegen unter
// chats/<projekt-id>/ im Zustandsverzeichnis.
package chatrun

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lazydev/internal/state"
)

// Status ist der Zustand eines Chats nach seiner letzten Runde.
type Status string

const (
	// StatusRunning gilt, solange eine Runde läuft.
	StatusRunning Status = "läuft"
	// StatusQuestion gilt nach einer fehlerfreien Runde, auf die Claude eine Antwort erwartet.
	StatusQuestion Status = "rückfrage"
	// StatusError gilt nach einer gescheiterten Runde.
	StatusError Status = "fehler"
	// StatusDone gilt nach einer fehlerfreien Runde ohne Rückfrage.
	StatusDone Status = "beendet"
)

// abortedText ist der Fehler einer Runde, deren Prozess ohne Ergebnis endete.
const abortedText = "Runde abgebrochen"

// Chat ist der Zustand eines Chats. Prompt und Shown gehören zur laufenden oder letzten Runde.
type Chat struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// Dir ist der Hauptordner des Projekts. Claude findet eine Sitzung nur dort.
	Dir       string  `json:"dir"`
	SessionID string  `json:"sessionId,omitempty"`
	CostUSD   float64 `json:"costUsd"`
	Status    Status  `json:"status"`
	PID       int     `json:"pid,omitempty"`
	Prompt    string  `json:"prompt"`
	// Shown steht statt Prompt im Verlauf, etwa statt eines langen eingebetteten Prompts.
	Shown     string `json:"shown"`
	Error     string `json:"error,omitempty"`
	StartedAt string `json:"startedAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Entry ist ein gespeicherter Chat mit Projekt und Chat-ID aus dem Dateipfad.
type Entry struct {
	ProjectID, ID string
	Chat          *Chat
}

// Effective liefert Status und Fehler für die Anzeige. Eine laufende Runde ohne lebenden
// Prozess gilt als abgebrochen.
func (c *Chat) Effective() (Status, string) {
	if c.Status == StatusRunning && (c.PID == 0 || !state.IsChat(c.PID)) {
		return StatusError, abortedText
	}
	return c.Status, c.Error
}

// Notifies meldet, ob der Chat als Benachrichtigung erscheint.
func (c *Chat) Notifies() bool {
	s, _ := c.Effective()
	return s == StatusQuestion || s == StatusError
}

// Before meldet, ob c vor o kommt: nach UpdatedAt, dann StartedAt.
func (c *Chat) Before(o *Chat) bool {
	if c.UpdatedAt != o.UpdatedAt {
		return c.UpdatedAt < o.UpdatedAt
	}
	return c.StartedAt < o.StartedAt
}

// lockName ist die Sperre für jede Lese-Schreib-Folge auf den Zustand eines Chats.
func lockName(projectID, id string) string {
	return "chat-" + projectID + "-" + id
}

// Load liefert fs.ErrNotExist, wenn es den Chat nicht gibt.
func Load(projectID, id string) (*Chat, error) {
	data, err := os.ReadFile(state.ChatPath(projectID, id))
	if err != nil {
		return nil, err
	}
	c := &Chat{}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// save schreibt atomar, damit die TUI nie eine halbe Datei liest.
func save(projectID, id string, c *Chat) error {
	path := state.ChatPath(projectID, id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".chat-*.json")
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

// Remove löscht den Zustand eines Chats. Das Log bleibt erhalten.
func Remove(projectID, id string) error {
	if err := os.Remove(state.ChatPath(projectID, id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// RemoveIfDone löscht den Zustand nur, wenn der Chat unter der Sperre noch beendet ist. Ein
// veralteter Stand löscht so keinen Chat, der inzwischen eine Runde startet.
func RemoveIfDone(projectID, id string) error {
	unlock, err := state.Lock(lockName(projectID, id))
	if err != nil {
		return err
	}
	defer unlock()
	c, err := Load(projectID, id)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if c.Status != StatusDone {
		return nil
	}
	return Remove(projectID, id)
}

// All liest die Chats aller Projekte, sortiert nach UpdatedAt, dann StartedAt. Unlesbare
// Dateien überspringt All.
func All() ([]Entry, error) {
	files, err := filepath.Glob(filepath.Join(state.Dir(), "chats", "*", "*.json"))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, f := range files {
		name := filepath.Base(f)
		// Temporäre Dateien von save beginnen mit einem Punkt.
		if strings.HasPrefix(name, ".") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		c := &Chat{}
		if json.Unmarshal(data, c) != nil {
			continue
		}
		entries = append(entries, Entry{
			ProjectID: filepath.Base(filepath.Dir(f)),
			ID:        strings.TrimSuffix(name, ".json"),
			Chat:      c,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Chat.Before(entries[j].Chat)
	})
	return entries, nil
}
