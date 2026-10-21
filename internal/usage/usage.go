// Package usage keeps the last known state of the Claude limits. Every call of claude -p reports
// it in stream-json as rate_limit_event.
package usage

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"lazydev/internal/fsutil"
	"lazydev/internal/state"
)

// Window is a limit window, e.g. five_hour or seven_day.
type Window struct {
	Name        string    `json:"name"`
	Utilization float64   `json:"utilization"`
	ResetsAt    time.Time `json:"resetsAt"`
}

// Snapshot is the state from one rate_limit_event.
type Snapshot struct {
	Time           time.Time `json:"time"`
	Status         string    `json:"status"`
	OverageStatus  string    `json:"overageStatus,omitempty"`
	IsUsingOverage bool      `json:"isUsingOverage,omitempty"`
	Windows        []Window  `json:"windows"`
}

type rawEvent struct {
	Type string `json:"type"`
	Info struct {
		Status         string `json:"status"`
		OverageStatus  string `json:"overageStatus"`
		IsUsingOverage bool   `json:"isUsingOverage"`
		UnifiedWindows map[string]struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    int64   `json:"resetsAt"`
		} `json:"unifiedWindows"`
	} `json:"rate_limit_info"`
}

// windowOrder sorts known windows by length, unknown ones after them by name.
var windowOrder = map[string]int{"five_hour": 0, "seven_day": 1}

// Parse reads a rate_limit_event. ok is false for other lines.
func Parse(line []byte, now time.Time) (s Snapshot, ok bool) {
	var e rawEvent
	if json.Unmarshal(line, &e) != nil || e.Type != "rate_limit_event" || len(e.Info.UnifiedWindows) == 0 {
		return Snapshot{}, false
	}
	s = Snapshot{Time: now, Status: e.Info.Status, OverageStatus: e.Info.OverageStatus, IsUsingOverage: e.Info.IsUsingOverage}
	for name, w := range e.Info.UnifiedWindows {
		s.Windows = append(s.Windows, Window{Name: name, Utilization: w.Utilization, ResetsAt: time.Unix(w.ResetsAt, 0)})
	}
	sort.Slice(s.Windows, func(i, j int) bool {
		oi, iok := windowOrder[s.Windows[i].Name]
		oj, jok := windowOrder[s.Windows[j].Name]
		switch {
		case iok && jok:
			return oi < oj
		case iok != jok:
			return iok
		}
		return s.Windows[i].Name < s.Windows[j].Name
	})
	return s, true
}

func path() string {
	return filepath.Join(state.Dir(), "usage.json")
}

var mu sync.Mutex

// Record saves the state from line if it is a rate_limit_event. It ignores write errors, the
// display is only a hint.
func Record(line []byte) {
	s, ok := Parse(line, time.Now())
	if !ok {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if data, err := json.MarshalIndent(s, "", "  "); err == nil {
		fsutil.WriteAtomic(path(), data, 0o644)
	}
}

// Load returns the last saved state. ok is false while there is none.
func Load() (s Snapshot, ok bool, err error) {
	data, err := os.ReadFile(path())
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, false, err
	}
	return s, true, nil
}
