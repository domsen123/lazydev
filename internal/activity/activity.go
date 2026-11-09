// Package activity keeps a log of what the user does in the TUI: keys with their pane and
// dialog, time in terminal tabs, actions, and the phase changes of runs as the TUI sees them. One
// JSON object per line in $XDG_STATE_HOME/lazydev/activity.jsonl. The log exists to read back a
// week of use and turn it into a plan; "activity": false in settings.json switches it off.
package activity

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"lazydev/internal/state"
)

// Event is one line of the log. Only the fields a kind needs are set.
type Event struct {
	Time time.Time `json:"t"`
	// Kind is one of the Kind constants.
	Kind string `json:"kind"`
	// Project is the folder name of the selected project, PRD the selected PRD and Phase its
	// phase (the run's, or the status line's without a run).
	Project string `json:"project,omitempty"`
	PRD     string `json:"prd,omitempty"`
	Phase   string `json:"phase,omitempty"`
	// From is the phase before a run event; empty when the TUI saw the run for the first time.
	From string `json:"from,omitempty"`
	// Pane is projects, center, explorer or details. Tab is the tab inside it: prds, files,
	// details, docker, log, md or terminal:<program>. Dialog is the open overlay, if any.
	Pane   string `json:"pane,omitempty"`
	Tab    string `json:"tab,omitempty"`
	Dialog string `json:"dialog,omitempty"`
	// Key is the key as Bubble Tea names it, or the mouse event.
	Key string `json:"key,omitempty"`
	// Text is the footer text of an action, "start" or "end" of a session, "enter" or "leave" of
	// a terminal tab.
	Text string `json:"text,omitempty"`
	Err  string `json:"err,omitempty"`
	// Count and Seconds come with "leave": keys sent to the tab and the active seconds in it.
	Count   int     `json:"count,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
	// Version is the lazydev version, with "start".
	Version string `json:"version,omitempty"`
}

const (
	KindSession  = "session"
	KindKey      = "key"
	KindMouse    = "mouse"
	KindTerminal = "terminal"
	KindAction   = "action"
	KindRun      = "run"
)

// Idle is the gap between two events after which the time in between does not count as active.
const Idle = 5 * time.Minute

var (
	mu      sync.Mutex
	enabled = true
	now     = time.Now
)

// Enable switches the log on or off for the process.
func Enable(on bool) {
	mu.Lock()
	defer mu.Unlock()
	enabled = on
}

// Path is the log file.
func Path() string { return filepath.Join(state.Dir(), "activity.jsonl") }

// Log appends e, with the current time if it has none. Errors are dropped: the log must never get
// in the way of the TUI.
func Log(e Event) {
	mu.Lock()
	defer mu.Unlock()
	if !enabled {
		return
	}
	if e.Time.IsZero() {
		e.Time = now()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// Read returns the events since the given time in file order. Lines that are no event are
// skipped; a missing file gives no events and no error.
func Read(since time.Time) ([]Event, error) {
	f, err := os.Open(Path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e Event
		if json.Unmarshal(line, &e) != nil || e.Kind == "" {
			continue
		}
		if e.Time.Before(since) {
			continue
		}
		events = append(events, e)
	}
	return events, sc.Err()
}

// bucket is where an event spends its time: the pane and tab, or the terminal program.
func (e Event) bucket() string {
	if e.Kind == KindTerminal {
		return e.Tab
	}
	if e.Tab == "" {
		return e.Pane
	}
	return e.Pane + "/" + e.Tab
}

// userMade is true for events the user caused by hand.
func (e Event) userMade() bool {
	return e.Kind == KindKey || e.Kind == KindMouse || e.Kind == KindAction
}

// Summary is what Report prints, kept as data so tests can check it.
type Summary struct {
	Sessions int
	Active   time.Duration
	Keys     int
	Clicks   int
	// Time is the active time per bucket, Count the keys and clicks there.
	Time  map[string]time.Duration
	Count map[string]int
	// KeyCount counts "<key> <bucket>".
	KeyCount map[string]int
	// Actions counts the footer texts of actions.
	Actions map[string]int
	// Runs counts run events per phase; Reaction holds how long the user took to do anything
	// after a run reached blocked, error or merged while the TUI was open.
	Runs     map[string]int
	Reaction map[string][]time.Duration
	// Days maps "2006-01-02" to active time and sessions.
	Days map[string]DaySummary
}

// DaySummary is one day of the Days table.
type DaySummary struct {
	Active   time.Duration
	Sessions int
}

// Summarize folds events into a Summary.
func Summarize(events []Event) Summary {
	s := Summary{
		Time: map[string]time.Duration{}, Count: map[string]int{}, KeyCount: map[string]int{},
		Actions: map[string]int{}, Runs: map[string]int{}, Reaction: map[string][]time.Duration{},
		Days: map[string]DaySummary{},
	}
	addActive := func(e Event, d time.Duration) {
		s.Active += d
		s.Time[e.bucket()] += d
		day := s.Days[e.Time.Format("2006-01-02")]
		day.Active += d
		s.Days[e.Time.Format("2006-01-02")] = day
	}
	for i, e := range events {
		switch e.Kind {
		case KindSession:
			if e.Text == "start" {
				s.Sessions++
				day := s.Days[e.Time.Format("2006-01-02")]
				day.Sessions++
				s.Days[e.Time.Format("2006-01-02")] = day
			}
			continue
		case KindTerminal:
			if e.Text == "leave" {
				addActive(e, time.Duration(e.Seconds*float64(time.Second)))
				s.Count[e.bucket()] += e.Count
				s.Keys += e.Count
			}
			continue
		case KindRun:
			s.Runs[e.Phase]++
			if e.From == "" || (e.Phase != "blocked" && e.Phase != "error" && e.Phase != "merged") {
				continue
			}
			for _, next := range events[i+1:] {
				if next.Kind == KindSession {
					break
				}
				if next.userMade() {
					s.Reaction[e.Phase] = append(s.Reaction[e.Phase], next.Time.Sub(e.Time))
					break
				}
			}
			continue
		case KindAction:
			// An action is the TUI's answer, not a place the user is at: it owns no time.
			s.Actions[e.Text]++
			continue
		case KindKey:
			s.Keys++
			s.Count[e.bucket()]++
			s.KeyCount[e.Key+" "+e.bucket()]++
		case KindMouse:
			s.Clicks++
			s.Count[e.bucket()]++
		}
		// The time until the next event counts as active in this bucket, up to Idle.
		if i+1 < len(events) {
			if gap := events[i+1].Time.Sub(e.Time); gap > 0 && gap < Idle {
				addActive(e, gap)
			}
		}
	}
	return s
}

// Report renders the summary of events since the given time as text for the terminal.
func Report(events []Event, since time.Time) string {
	s := Summarize(events)
	var b strings.Builder
	fmt.Fprintf(&b, "Activity since %s: %d sessions, %s active, %d keys, %d clicks\n",
		since.Format("2006-01-02"), s.Sessions, dur(s.Active), s.Keys, s.Clicks)
	if len(events) == 0 {
		fmt.Fprintf(&b, "\nNothing logged yet. The TUI writes %s while it runs.\n", Path())
		return b.String()
	}

	b.WriteString("\nWhere the time went\n")
	for _, k := range sortedBy(s.Time, func(k string) float64 { return float64(s.Time[k]) }) {
		fmt.Fprintf(&b, "  %-28s %10s %6d keys\n", k, dur(s.Time[k]), s.Count[k])
	}

	if len(s.KeyCount) > 0 {
		b.WriteString("\nKeys outside terminals\n")
		keys := sortedBy(s.KeyCount, func(k string) float64 { return float64(s.KeyCount[k]) })
		for _, k := range keys[:min(15, len(keys))] {
			key, bucket, _ := strings.Cut(k, " ")
			fmt.Fprintf(&b, "  %-12s %-28s %5d\n", key, bucket, s.KeyCount[k])
		}
	}

	if len(s.Actions) > 0 {
		b.WriteString("\nActions\n")
		for _, k := range sortedBy(s.Actions, func(k string) float64 { return float64(s.Actions[k]) }) {
			fmt.Fprintf(&b, "  %4d  %s\n", s.Actions[k], k)
		}
	}

	if len(s.Runs) > 0 {
		b.WriteString("\nRuns seen by the TUI\n")
		for _, phase := range sortedBy(s.Runs, func(k string) float64 { return float64(s.Runs[k]) }) {
			fmt.Fprintf(&b, "  %-12s %4d", phase, s.Runs[phase])
			if r := s.Reaction[phase]; len(r) > 0 {
				sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
				fmt.Fprintf(&b, "   you reacted after %s (median), %s at most, %d times", dur(r[len(r)/2]), dur(r[len(r)-1]), len(r))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\nDays\n")
	days := make([]string, 0, len(s.Days))
	for d := range s.Days {
		days = append(days, d)
	}
	sort.Strings(days)
	for _, d := range days {
		day := s.Days[d]
		t, _ := time.Parse("2006-01-02", d)
		fmt.Fprintf(&b, "  %s %s %10s   %d sessions\n", t.Format("Mon"), d, dur(day.Active), day.Sessions)
	}
	return b.String()
}

// sortedBy returns the keys of m sorted by weight, highest first, ties by name.
func sortedBy[V any](m map[string]V, weight func(string) float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		wi, wj := weight(keys[i]), weight(keys[j])
		if wi != wj {
			return wi > wj
		}
		return keys[i] < keys[j]
	})
	return keys
}

// dur formats a duration as "2 h 05 min", "12 min" or "40 s".
func dur(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%d h %02d min", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	}
}
