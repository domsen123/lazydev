package runner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"lazydev/internal/config"
	"lazydev/internal/notify"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// webhook is a Discord stand-in that answers every request with code and keeps the titles of the
// embeds.
type webhook struct {
	*httptest.Server
	mu     sync.Mutex
	titles []string
}

func newWebhook(t *testing.T, code int) *webhook {
	t.Helper()
	old := notify.RetryDelay
	notify.RetryDelay = 0
	t.Cleanup(func() { notify.RetryDelay = old })
	w := &webhook{}
	w.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p struct {
			Embeds []struct {
				Title  string `json:"title"`
				Fields []struct {
					Name string `json:"name"`
				} `json:"fields"`
			} `json:"embeds"`
		}
		json.Unmarshal(body, &p)
		title := ""
		if len(p.Embeds) == 1 {
			title = p.Embeds[0].Title
			if len(p.Embeds[0].Fields) != 5 {
				t.Errorf("fields of %q: %+v", title, p.Embeds[0].Fields)
			}
		}
		w.mu.Lock()
		w.titles = append(w.titles, title)
		w.mu.Unlock()
		rw.WriteHeader(code)
	}))
	t.Cleanup(w.Close)
	return w
}

func (w *webhook) got() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.titles...)
}

// enableDiscord writes settings.json with the global switch on and url.
func enableDiscord(t *testing.T, d config.Discord) {
	t.Helper()
	if err := config.SaveSettings(config.Settings{Discord: d}); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyMerged(t *testing.T) {
	target := newProject(t, "", false)
	w := newWebhook(t, http.StatusNoContent)
	enableDiscord(t, config.Discord{Enabled: true, URL: w.URL})
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if got := w.got(); len(got) != 1 || got[0] != "✓ Merged: auth/new-token" {
		t.Errorf("requests: %q", got)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepMerge)); !strings.Contains(log, "Discord notification sent: merged") {
		t.Errorf("merge log:\n%s", log)
	}
}

func TestNotifyFailed(t *testing.T) {
	target := newProject(t, "", false)
	w := newWebhook(t, http.StatusNoContent)
	enableDiscord(t, config.Discord{Enabled: true, URL: w.URL})
	t.Setenv("FAKE_CLAUDE_MODE", "exit")
	if err := Run(context.Background(), target, prdName); err == nil {
		t.Fatal("Run succeeded")
	}
	if got := w.got(); len(got) != 1 || got[0] != "✗ Failed: auth/new-token" {
		t.Errorf("requests: %q", got)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepImplement)); !strings.Contains(log, "Discord notification sent: failed") {
		t.Errorf("implement log:\n%s", log)
	}
}

func TestNotifyAborted(t *testing.T) {
	target := newProject(t, "", false)
	w := newWebhook(t, http.StatusNoContent)
	enableDiscord(t, config.Discord{Enabled: true, URL: w.URL})
	t.Setenv("FAKE_CLAUDE_MODE", "sleep")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, target, prdName) }()
	waitFor(t, "the implement step", func() bool {
		r, err := state.Load("app", prdName)
		return err == nil && r.Phase == project.PhaseImplement
	})
	cancel()
	if err := <-done; err == nil || err.Error() != "implement aborted" {
		t.Fatalf("Run: %v", err)
	}
	if got := w.got(); len(got) != 1 || got[0] != "■ Aborted: auth/new-token" {
		t.Errorf("requests: %q", got)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepImplement)); !strings.Contains(log, "Discord notification sent: aborted") {
		t.Errorf("implement log:\n%s", log)
	}
}

func TestNotifyMainFolder(t *testing.T) {
	target := newProject(t, "", false)
	w := newWebhook(t, http.StatusNoContent)
	enableDiscord(t, config.Discord{Enabled: true, URL: w.URL})
	write(t, target.Dir, "CLAUDE.md", "# app\nlocal change\n")
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), target, prdName) }()
	waitFor(t, "the merge queue", func() bool {
		r, err := state.Load("app", prdName)
		return err == nil && r.Note == "main folder: changed: CLAUDE.md"
	})
	waitFor(t, "the waiting notification", func() bool { return len(w.got()) == 1 })
	runGit(t, target.Dir, "checkout", "--", "CLAUDE.md")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := w.got(); len(got) != 2 || got[0] != "◐ Waiting for main folder: auth/new-token" || got[1] != "✓ Merged: auth/new-token" {
		t.Errorf("requests: %q", got)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepMerge)); !strings.Contains(log, "Discord notification sent: main folder") {
		t.Errorf("merge log:\n%s", log)
	}
}

func TestNotifySwitchedOff(t *testing.T) {
	off := false
	for name, d := range map[string]func(url, dir string) config.Discord{
		"global off": func(url, dir string) config.Discord { return config.Discord{URL: url} },
		"project off": func(url, dir string) config.Discord {
			return config.Discord{Enabled: true, URL: url, Projects: map[string]config.DiscordProject{dir: {Enabled: &off}}}
		},
		"no URL": func(url, dir string) config.Discord { return config.Discord{Enabled: true} },
	} {
		t.Run(name, func(t *testing.T) {
			target := newProject(t, "", false)
			w := newWebhook(t, http.StatusNoContent)
			enableDiscord(t, d(w.URL, target.Dir))
			if err := Run(context.Background(), target, prdName); err != nil {
				t.Fatal(err)
			}
			if got := w.got(); len(got) != 0 {
				t.Errorf("requests: %q", got)
			}
			if log := read(t, state.LogPath("app", prdName, state.StepMerge)); strings.Contains(log, "Discord") {
				t.Errorf("merge log:\n%s", log)
			}
		})
	}
}

func TestNotifyDeliveryFails(t *testing.T) {
	target := newProject(t, "", false)
	w := newWebhook(t, http.StatusInternalServerError)
	enableDiscord(t, config.Discord{Enabled: true, URL: w.URL})
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if got := w.got(); len(got) != 4 {
		t.Errorf("requests: %q", got)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged || r.Error != nil || r.Note != "" {
		t.Errorf("state: %+v", r)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepMerge)); !strings.Contains(log, "Discord notification failed: discord answered 500") {
		t.Errorf("merge log:\n%s", log)
	}
}
