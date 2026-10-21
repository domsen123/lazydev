// Package state liest und schreibt den Lauf-Zustand unter ~/.local/state/lazydev.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazydev/internal/project"
)

type Step struct {
	Name      string `json:"name"`
	SessionID string `json:"sessionId,omitempty"`
	ExitCode  *int   `json:"exitCode"`
	// OK ist false, wenn der Schritt trotz Exit-Code 0 scheiterte, etwa an einer offenen Frage.
	OK        bool     `json:"ok"`
	CostUSD   *float64 `json:"costUsd,omitempty"`
	StartedAt string   `json:"startedAt,omitempty"`
	EndedAt   string   `json:"endedAt,omitempty"`
}

// Run ist der Zustand eines PRDs zwischen Start und Aufräumen.
type Run struct {
	PRD    string `json:"prd"`
	Branch string `json:"branch"`
	// Base ist der Ziel-Branch, von dem der Worktree abzweigt und in den der Merge geht.
	Base      string        `json:"base,omitempty"`
	Worktree  string        `json:"worktree"`
	Port      int           `json:"port"`
	Slug      string        `json:"slug"`
	Phase     project.Phase `json:"phase"`
	PID       int           `json:"pid"`
	StartedAt string        `json:"startedAt"`
	Steps     []Step        `json:"steps"`
	Error     *string       `json:"error"`
	// Note ist ein Hinweis an den Menschen, z. B. „Hauptordner nicht sauber“.
	Note string `json:"note,omitempty"`
	// MergeApproved gibt den Merge frei, wenn autoMerge aus ist.
	MergeApproved bool `json:"mergeApproved,omitempty"`
}

// Dir beachtet XDG_STATE_HOME.
func Dir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "lazydev")
}

func RunsDir(projectID string) string {
	return filepath.Join(Dir(), "runs", projectID)
}

func Path(projectID, prdName string) string {
	return filepath.Join(RunsDir(projectID), prdName+".json")
}

// LogPath ist das Log eines Schritts, z. B. "setup" oder "implement".
func LogPath(projectID, prdName, step string) string {
	return filepath.Join(RunsDir(projectID), prdName+"."+step+".jsonl")
}

// RunnerLogPath nimmt stdout und stderr eines losgelösten Runners auf, auch Abstürze.
func RunnerLogPath(projectID, prdName string) string {
	return filepath.Join(RunsDir(projectID), prdName+".runner.log")
}

// ChatsDir enthält Zustand und Log der Chats eines Projekts.
func ChatsDir(projectID string) string {
	return filepath.Join(Dir(), "chats", projectID)
}

// ChatPath ist der Zustand eines Chats mit /create-prd.
func ChatPath(projectID, chatID string) string {
	return filepath.Join(ChatsDir(projectID), chatID+".json")
}

// ChatLogPath ist das Log eines Chats mit /create-prd.
func ChatLogPath(projectID, chatID string) string {
	return filepath.Join(ChatsDir(projectID), chatID+".jsonl")
}

// IssuesPath ist der Zwischenspeicher der GitHub-Issues eines Projekts.
func IssuesPath(projectID string) string {
	return filepath.Join(Dir(), "issues", projectID+".json")
}

// Load liefert fs.ErrNotExist, wenn es keinen Lauf gibt.
func Load(projectID, prdName string) (*Run, error) {
	data, err := os.ReadFile(Path(projectID, prdName))
	if err != nil {
		return nil, err
	}
	r := &Run{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Save schreibt atomar, damit die TUI nie eine halbe Datei liest.
func Save(projectID, prdName string, r *Run) error {
	path := Path(projectID, prdName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".run-*.json")
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

// Remove löscht Zustand und Logs eines PRDs.
func Remove(projectID, prdName string) error {
	logs, err := filepath.Glob(filepath.Join(RunsDir(projectID), prdName+".*.jsonl"))
	if err != nil {
		return err
	}
	for _, f := range append(logs, Path(projectID, prdName), RunnerLogPath(projectID, prdName)) {
		// Ein Schrittname enthält keinen Punkt. So bleibt "<prd>.x.<schritt>.jsonl" eines PRDs
		// mit Punkt im Namen unberührt.
		step := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), prdName+"."), ".jsonl")
		if strings.Contains(step, ".") {
			continue
		}
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// UsedPorts sammelt die Ports aller gespeicherten Läufe über alle Projekte.
func UsedPorts() (map[int]bool, error) {
	ports := map[int]bool{}
	files, err := filepath.Glob(filepath.Join(Dir(), "runs", "*", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var r Run
		// Ein gemergter Lauf hat keinen Worktree mehr und gibt seinen Port frei.
		if json.Unmarshal(data, &r) == nil && r.Port > 0 && r.Phase != project.PhaseMerged {
			ports[r.Port] = true
		}
	}
	return ports, nil
}

// Now liefert den Zeitstempel im Format des Lauf-Zustands.
func Now() string {
	return time.Now().Format(time.RFC3339)
}

func (r *Run) StartStep(name string) {
	r.Steps = append(r.Steps, Step{Name: name, StartedAt: Now()})
}

// EndStep schließt den zuletzt gestarteten Schritt ab.
func (r *Run) EndStep(exitCode int, ok bool) {
	s := r.LastStep()
	if s == nil {
		return
	}
	s.ExitCode = &exitCode
	s.OK = ok
	s.EndedAt = Now()
}

// LastStep liefert nil, solange kein Schritt gestartet ist.
func (r *Run) LastStep() *Step {
	if len(r.Steps) == 0 {
		return nil
	}
	return &r.Steps[len(r.Steps)-1]
}

func (r *Run) Fail(err error) {
	msg := err.Error()
	r.Error = &msg
	r.Phase = project.PhaseError
}

// Cost summiert die Kosten aller Schritte in USD.
func (r *Run) Cost() float64 {
	total := 0.0
	for _, s := range r.Steps {
		if s.CostUSD != nil {
			total += *s.CostUSD
		}
	}
	return total
}
