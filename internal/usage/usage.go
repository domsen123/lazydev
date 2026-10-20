// Package usage hält den letzten bekannten Stand der Claude-Limits. Jeder Aufruf von claude -p
// meldet ihn im stream-json als Ereignis rate_limit_event.
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

	"lazydev/internal/state"
)

// Window ist ein Limit-Fenster, z. B. five_hour oder seven_day.
type Window struct {
	Name        string    `json:"name"`
	Utilization float64   `json:"utilization"`
	ResetsAt    time.Time `json:"resetsAt"`
}

// Snapshot ist der Stand aus einem rate_limit_event.
type Snapshot struct {
	Time           time.Time `json:"time"`
	Status         string    `json:"status"`
	OverageStatus  string    `json:"overageStatus,omitempty"`
	OverageReason  string    `json:"overageReason,omitempty"`
	IsUsingOverage bool      `json:"isUsingOverage,omitempty"`
	Windows        []Window  `json:"windows"`
}

type rawEvent struct {
	Type string `json:"type"`
	Info struct {
		Status                string `json:"status"`
		OverageStatus         string `json:"overageStatus"`
		OverageDisabledReason string `json:"overageDisabledReason"`
		IsUsingOverage        bool   `json:"isUsingOverage"`
		UnifiedWindows        map[string]struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    int64   `json:"resetsAt"`
		} `json:"unifiedWindows"`
	} `json:"rate_limit_info"`
}

// windowOrder sortiert bekannte Fenster nach Länge, unbekannte dahinter nach Namen.
var windowOrder = map[string]int{"five_hour": 0, "seven_day": 1}

// Parse liest ein rate_limit_event. ok ist false für andere Zeilen.
func Parse(line []byte, now time.Time) (s Snapshot, ok bool) {
	var e rawEvent
	if json.Unmarshal(line, &e) != nil || e.Type != "rate_limit_event" || len(e.Info.UnifiedWindows) == 0 {
		return Snapshot{}, false
	}
	s = Snapshot{
		Time:           now,
		Status:         e.Info.Status,
		OverageStatus:  e.Info.OverageStatus,
		OverageReason:  e.Info.OverageDisabledReason,
		IsUsingOverage: e.Info.IsUsingOverage,
	}
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

// Window liefert das Fenster mit dem Namen oder ok=false.
func (s Snapshot) Window(name string) (Window, bool) {
	for _, w := range s.Windows {
		if w.Name == name {
			return w, true
		}
	}
	return Window{}, false
}

func Path() string {
	return filepath.Join(state.Dir(), "usage.json")
}

var mu sync.Mutex

// Record speichert den Stand aus line, falls die Zeile ein rate_limit_event ist. Fehler beim
// Schreiben ignoriert Record, die Anzeige ist nur ein Hinweis.
func Record(line []byte) {
	s, ok := Parse(line, time.Now())
	if !ok {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(Path()), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(Path()), ".usage-*.json")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return
	}
	if tmp.Close() == nil {
		os.Rename(tmp.Name(), Path())
	}
}

// Load liefert den letzten gespeicherten Stand. ok ist false, solange es keinen gibt.
func Load() (s Snapshot, ok bool, err error) {
	data, err := os.ReadFile(Path())
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
