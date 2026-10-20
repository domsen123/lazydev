package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"lazydev/internal/project"
)

// Entry ist ein gespeicherter Lauf mit Projekt und PRD-Namen aus dem Dateipfad.
type Entry struct {
	ProjectID string
	PRDName   string
	Run       *Run
}

// All liest die Läufe aller Projekte, sortiert nach StartedAt. Unlesbare Dateien überspringt All.
func All() ([]Entry, error) {
	files, err := filepath.Glob(filepath.Join(Dir(), "runs", "*", "*.json"))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		r := &Run{}
		if json.Unmarshal(data, r) != nil {
			continue
		}
		entries = append(entries, Entry{
			ProjectID: filepath.Base(filepath.Dir(f)),
			PRDName:   strings.TrimSuffix(filepath.Base(f), ".json"),
			Run:       r,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Run.StartedAt < entries[j].Run.StartedAt
	})
	return entries, nil
}

// Running meldet, ob der Runner des Laufs noch lebt.
func (r *Run) Running() bool {
	return r.PID > 0 && IsRunner(r.PID)
}

// Active meldet Phasen, in denen ein Runner arbeitet.
func Active(p project.Phase) bool {
	switch p {
	case project.PhaseSetup, project.PhaseImplement, project.PhaseReview, project.PhaseMerge:
		return true
	}
	return false
}

// Effective liefert die Phase für die Anzeige: Ein aktiver Lauf ohne lebenden Runner gilt
// als abgebrochen.
func (r *Run) Effective() project.Phase {
	if Active(r.Phase) && !r.Running() {
		return project.PhaseAborted
	}
	return r.Phase
}

// IsRunner meldet, ob pid lebt und zu "lazydev run" gehört. So zählt eine wiederverwendete
// PID eines fremden Prozesses nicht als Runner.
func IsRunner(pid int) bool {
	return isLazydev(pid, "run")
}

// IsChat meldet, ob pid lebt und zu "lazydev chat" gehört.
func IsChat(pid int) bool {
	return isLazydev(pid, "chat")
}

// isLazydev meldet, ob pid lebt und "lazydev <sub>" ausführt.
func isLazydev(pid int, sub string) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return false
	}
	args := strings.Split(string(bytes.TrimRight(data, "\x00")), "\x00")
	return len(args) >= 2 && strings.HasPrefix(filepath.Base(args[0]), "lazydev") && args[1] == sub
}

// TryLock sperrt wie Lock, wartet aber nicht. ok ist false, wenn ein anderer Prozess die
// Sperre hält.
func TryLock(name string) (unlock func(), ok bool, err error) {
	f, err := openLock(name)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, true, nil
}

func openLock(name string) (*os.File, error) {
	path := filepath.Join(Dir(), "locks", name+".lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
}

// Lock sperrt die Datei locks/<name>.lock mit flock, bis unlock läuft. Lock wartet, solange
// ein anderer Prozess die Sperre hält.
func Lock(name string) (unlock func(), err error) {
	f, err := openLock(name)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
