package activity

import (
	"os"
	"strings"
	"testing"
	"time"
)

func useTempState(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Cleanup(func() { Enable(true) })
}

func TestLogRoundTrip(t *testing.T) {
	useTempState(t)
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	Log(Event{Time: at, Kind: KindSession, Text: "start", Version: "test"})
	Log(Event{Time: at.Add(time.Second), Kind: KindKey, Key: "j", Pane: "explorer", Tab: "prds", Project: "catd"})
	// A broken line never breaks reading.
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{not json\n")
	f.Close()
	Log(Event{Time: at.Add(2 * time.Second), Kind: KindAction, Text: "started x"})

	events, err := Read(time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Kind != KindSession || events[1].Key != "j" || events[2].Text != "started x" {
		t.Fatalf("events = %+v", events)
	}
	since, err := Read(at.Add(2 * time.Second))
	if err != nil || len(since) != 1 {
		t.Fatalf("since = %+v, %v", since, err)
	}
	info, err := os.Stat(Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
}

func TestDisabledWritesNothing(t *testing.T) {
	useTempState(t)
	Enable(false)
	Log(Event{Kind: KindKey, Key: "j"})
	if _, err := os.Stat(Path()); !os.IsNotExist(err) {
		t.Fatalf("file exists: %v", err)
	}
	events, err := Read(time.Time{})
	if err != nil || events != nil {
		t.Fatalf("events = %v, %v", events, err)
	}
}

func TestSummarize(t *testing.T) {
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	step := func(s int) time.Time { return at.Add(time.Duration(s) * time.Second) }
	events := []Event{
		{Time: step(0), Kind: KindSession, Text: "start"},
		{Time: step(1), Kind: KindKey, Key: "j", Pane: "explorer", Tab: "prds"},
		{Time: step(3), Kind: KindKey, Key: "j", Pane: "explorer", Tab: "prds"},
		{Time: step(4), Kind: KindKey, Key: "s", Pane: "explorer", Tab: "prds"},
		{Time: step(5), Kind: KindAction, Text: "started x"},
		{Time: step(6), Kind: KindTerminal, Text: "enter", Tab: "terminal:claude"},
		{Time: step(100), Kind: KindTerminal, Text: "leave", Tab: "terminal:claude", Count: 40, Seconds: 80},
		{Time: step(100), Kind: KindKey, Key: "k", Pane: "explorer", Tab: "prds"},
		// The run blocks; the user does something 30 s later.
		{Time: step(110), Kind: KindRun, PRD: "x", Phase: "blocked", From: "implement"},
		{Time: step(140), Kind: KindMouse, Key: "left press", Pane: "center", Tab: "log"},
		// Seen at start, no reaction measured.
		{Time: step(141), Kind: KindRun, PRD: "y", Phase: "merged"},
		// A merged run right before the end of the session: no reaction either.
		{Time: step(150), Kind: KindRun, PRD: "x", Phase: "merged", From: "merge"},
		{Time: step(151), Kind: KindSession, Text: "end"},
		// An hour later: the gap is idle.
		{Time: step(3800), Kind: KindSession, Text: "start"},
		{Time: step(3801), Kind: KindKey, Key: "q", Pane: "projects"},
	}
	s := Summarize(events)
	if s.Sessions != 2 || s.Keys != 45 || s.Clicks != 1 {
		t.Fatalf("sessions=%d keys=%d clicks=%d", s.Sessions, s.Keys, s.Clicks)
	}
	if s.Time["terminal:claude"] != 80*time.Second || s.Count["terminal:claude"] != 40 {
		t.Fatalf("terminal = %v, %d", s.Time["terminal:claude"], s.Count["terminal:claude"])
	}
	// j→j 2 s, j→s 1 s, s→action 1 s, k→run 10 s; the action and the run count no time of their own.
	if s.Time["explorer/prds"] != 14*time.Second {
		t.Fatalf("explorer/prds = %v", s.Time["explorer/prds"])
	}
	if s.KeyCount["j explorer/prds"] != 2 || s.Actions["started x"] != 1 {
		t.Fatalf("keys = %v, actions = %v", s.KeyCount, s.Actions)
	}
	if s.Runs["blocked"] != 1 || s.Runs["merged"] != 2 {
		t.Fatalf("runs = %v", s.Runs)
	}
	if r := s.Reaction["blocked"]; len(r) != 1 || r[0] != 30*time.Second {
		t.Fatalf("reaction blocked = %v", r)
	}
	if r := s.Reaction["merged"]; len(r) != 0 {
		t.Fatalf("reaction merged = %v", r)
	}
	if d := s.Days["2026-10-29"]; d.Sessions != 2 || d.Active != s.Active {
		t.Fatalf("day = %+v, active = %v", d, s.Active)
	}

	out := Report(events, at)
	for _, want := range []string{"2 sessions", "terminal:claude", "you reacted after 30 s", "started x", "Thu 2026-10-29"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(Report(nil, at), "Nothing logged yet") {
		t.Errorf("empty report:\n%s", Report(nil, at))
	}
}
