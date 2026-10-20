package chatrun

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lazydev/internal/config"
	"lazydev/internal/state"
)

// fakeClaude antwortet wie claude -p und protokolliert die Argumente. FAKE_CLAUDE_MODE steuert
// das Ende: question, done, exit, noresult oder iserror.
func fakeClaude(t *testing.T) (bin, args string) {
	t.Helper()
	dir := t.TempDir()
	args = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s|' \"$@\" >> " + args + "\necho >> " + args + "\n" +
		`echo '{"type":"system","subtype":"init","session_id":"sess-1"}'` + "\n" +
		`case "$FAKE_CLAUDE_MODE" in` + "\n" +
		// printf statt echo, weil echo in dash "\n" als Zeilenumbruch ausgibt.
		`question) printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.25,"result":"Welche Variante?\n\n- A: Profil\n- B: Anmeldung"}' ;;` + "\n" +
		`done) echo '{"type":"result","is_error":false,"total_cost_usd":0.25,"result":"PRD gespeichert unter prd/x.md."}' ;;` + "\n" +
		`exit) exit 3 ;;` + "\n" +
		`noresult) ;;` + "\n" +
		`iserror) printf '%s\n' '{"type":"result","is_error":true,"total_cost_usd":0.25,"result":"Limit erreicht\nDetails"}' ;;` + "\n" +
		`esac` + "\n"
	bin = filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, args
}

// setup isoliert das Zustandsverzeichnis und lässt jede Runde in einer Goroutine laufen.
// wait wartet auf alle gestarteten Runden.
func setup(t *testing.T) (cfg *config.Config, args string, wait func()) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	bin, args := fakeClaude(t)
	cfg = &config.Config{Claude: config.Claude{Binary: bin}}
	var wg sync.WaitGroup
	restore := SetSpawn(func(projectID, id string) (int, error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Round(cfg, projectID, id); err != nil {
				t.Errorf("Round: %v", err)
			}
		}()
		return 0, nil
	})
	t.Cleanup(func() {
		wg.Wait()
		restore()
	})
	return cfg, args, wg.Wait
}

func newChat(t *testing.T) string {
	t.Helper()
	id, err := Create("alpha", "Neues PRD (Einfach): alpha", "Avatar-Upload", t.TempDir(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRoundStatus(t *testing.T) {
	tests := []struct {
		mode   string
		status Status
		err    string
	}{
		{"question", StatusQuestion, ""},
		{"done", StatusDone, ""},
		{"exit", StatusError, "claude endete mit Exit-Code 3"},
		{"noresult", StatusError, "Claude lieferte kein Ergebnis"},
		{"iserror", StatusError, "Claude meldet einen Fehler: Limit erreicht"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			_, _, wait := setup(t)
			t.Setenv("FAKE_CLAUDE_MODE", tt.mode)
			id := newChat(t)
			if err := Send("alpha", id, "Avatar-Upload", "/create-prd Avatar-Upload"); err != nil {
				t.Fatal(err)
			}
			wait()
			c, err := Load("alpha", id)
			if err != nil {
				t.Fatal(err)
			}
			if c.Status != tt.status || c.Error != tt.err || c.PID != 0 {
				t.Errorf("Zustand: %+v", c)
			}
			log, _ := os.ReadFile(state.ChatLogPath("alpha", id))
			if tt.err != "" && !strings.Contains(string(log), `"type":"lazydev","text":"`+tt.err) {
				t.Errorf("Log ohne Fehlerzeile:\n%s", log)
			}
		})
	}
}

func TestRoundResume(t *testing.T) {
	_, args, wait := setup(t)
	t.Setenv("FAKE_CLAUDE_MODE", "question")
	id := newChat(t)
	if err := Send("alpha", id, "Avatar-Upload", "/create-prd Avatar-Upload"); err != nil {
		t.Fatal(err)
	}
	wait()
	if err := Send("alpha", id, "2 MB", "2 MB"); err != nil {
		t.Fatal(err)
	}
	wait()

	c, _ := Load("alpha", id)
	if c.SessionID != "sess-1" || c.CostUSD < 0.49 || c.Status != StatusQuestion || c.Prompt != "2 MB" {
		t.Errorf("Zustand: %+v", c)
	}
	got, _ := os.ReadFile(args)
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 || strings.Contains(lines[0], "--resume") || !strings.Contains(lines[1], "--resume|sess-1|") ||
		!strings.Contains(lines[0], "--model|opus|") {
		t.Errorf("Aufrufe von claude:\n%s", got)
	}
	log, _ := os.ReadFile(state.ChatLogPath("alpha", id))
	if !strings.Contains(string(log), `"type":"user_prompt","text":"/create-prd Avatar-Upload","shown":"Avatar-Upload"`) ||
		!strings.Contains(string(log), `{"ts":"`) {
		t.Errorf("Log:\n%s", log)
	}
}

func TestRoundWithoutState(t *testing.T) {
	cfg, args, _ := setup(t)
	id := newChat(t)
	if err := Remove("alpha", id); err != nil {
		t.Fatal(err)
	}
	if err := Round(cfg, "alpha", id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state.ChatPath("alpha", id)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Round legt den Zustand neu an: %v", err)
	}
	if _, err := os.Stat(args); err == nil {
		t.Error("Round startet claude ohne Zustand")
	}
}

// Ein Chat, den Stop während der Runde löscht, bleibt gelöscht.
func TestRoundRemovedDuringRound(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	os.WriteFile(bin, []byte("#!/bin/sh\nsleep 0.3\necho '{\"type\":\"result\",\"is_error\":false,\"result\":\"ok\"}'\n"), 0o755)
	cfg := &config.Config{Claude: config.Claude{Binary: bin}}
	id := newChat(t)
	done := make(chan error)
	restore := SetSpawn(func(projectID, id string) (int, error) {
		go func() { done <- Round(cfg, projectID, id) }()
		return 0, nil
	})
	defer restore()
	if err := Send("alpha", id, "x", "x"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := Stop("alpha", id); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state.ChatPath("alpha", id)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Zustand nach Stop: %v", err)
	}
}

func TestEffectiveAborted(t *testing.T) {
	c := &Chat{Status: StatusRunning, PID: 999999999}
	if s, text := c.Effective(); s != StatusError || text != "Runde abgebrochen" || !c.Notifies() {
		t.Errorf("Effective: %v %q", s, text)
	}
	c = &Chat{Status: StatusDone}
	if s, _ := c.Effective(); s != StatusDone || c.Notifies() {
		t.Errorf("Effective für beendet: %v", s)
	}
}

func TestCreateSuffix(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	now := time.Date(2026, 10, 20, 14, 3, 5, 0, time.Local)
	var ids []string
	for range 3 {
		id, err := Create("alpha", "t", "d", "/tmp", now)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	want := []string{"2026-10-20-140305", "2026-10-20-140305-2", "2026-10-20-140305-3"}
	if strings.Join(ids, " ") != strings.Join(want, " ") {
		t.Errorf("IDs: %v", ids)
	}
	c, err := Load("alpha", ids[0])
	if err != nil || c.Status != StatusDone || c.Description != "d" || c.Dir != "/tmp" {
		t.Errorf("Zustand nach Create: %+v %v", c, err)
	}
	all, _ := All()
	if len(all) != 3 || all[0].ProjectID != "alpha" {
		t.Errorf("All: %+v", all)
	}
}

func TestRemoveIfDone(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	id := newChat(t)
	if _, err := update("alpha", id, func(c *Chat) { c.Status = StatusQuestion }); err != nil {
		t.Fatal(err)
	}
	RemoveIfDone("alpha", id)
	if _, err := Load("alpha", id); err != nil {
		t.Errorf("RemoveIfDone löscht eine Rückfrage: %v", err)
	}
	update("alpha", id, func(c *Chat) { c.Status = StatusDone })
	RemoveIfDone("alpha", id)
	if _, err := Load("alpha", id); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("RemoveIfDone lässt einen beendeten Chat stehen: %v", err)
	}
}
