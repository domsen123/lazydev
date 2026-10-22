package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/claude"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

const (
	blockedPRDPath = "prd/auth/token.md"
	blockedPRD     = "# PRD: Token\n\n> Status: blockiert (2026-10-22) · Scope: x\n\nbody\n\n## Blocker\n\nSchritt: implement\n\n- Feld fehlt\n"
)

// blockedModel is cockpitModel with the PRD auth/token whose run is blocked. The PRD lies in the
// worktree of the run, the PRD list reads it from there. The PRDs tab has the focus.
func blockedModel(t *testing.T) (model, *projectEntry, string) {
	t.Helper()
	m := cockpitModel(t)
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "prd", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, blockedPRDPath), []byte(blockedPRD), 0o644); err != nil {
		t.Fatal(err)
	}
	failed := "implement blocked: Feld fehlt"
	p := m.projects[0]
	p.runs = map[string]*state.Run{"token": {
		Phase: project.PhaseBlocked, Worktree: wt, PRD: blockedPRDPath, Branch: "feat/token", Base: "main",
		StartedAt: "2026-10-22T10:00:00Z", Error: &failed,
		Steps: []state.Step{{Name: state.StepSetup, ExitCode: new(int), OK: true}, {Name: state.StepImplement, ExitCode: new(int)}},
	}}
	d, err := project.Read(wt, filepath.Join(wt, blockedPRDPath))
	if err != nil {
		t.Fatal(err)
	}
	p.prds = []project.PRD{d}
	m.focus, m.rightTab, m.bus = colRight, rightPRDs, &sender{}
	return m, p, wt
}

// fakeClaude replaces claude with a shell that writes its folder and arguments to the returned
// file and keeps running.
func fakeClaude(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "claude.out")
	old := claude.Command
	claude.Command = []string{"sh", "-c", `{ pwd -P; printf '%s\n' "$@"; } > "` + out + `"; sleep 30`, "sh"}
	t.Cleanup(func() { claude.Command = old })
	return out
}

func hangupAll(t *testing.T, p *projectEntry) {
	t.Cleanup(func() {
		for _, term := range p.terminals() {
			term.Hangup()
			<-term.Done()
			term.Release()
		}
	})
}

func TestUnblockOpensClaudeInWorktree(t *testing.T) {
	m, p, wt := blockedModel(t)
	out := fakeClaude(t)
	m = press(t, m, keyRunes("f"))
	hangupAll(t, p)
	if m.err != "" {
		t.Fatalf("err: %q", m.err)
	}
	term := p.activeTerminal()
	if term == nil || term.TabTitle() != "unblock" || m.focus != colCenter {
		t.Fatalf("tab %v, focus %d", term, m.focus)
	}
	var got string
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(got, "--agents") {
		if time.Now().After(deadline) {
			t.Fatalf("claude output: %q", got)
		}
		time.Sleep(10 * time.Millisecond)
		data, _ := os.ReadFile(out)
		got = string(data)
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	realWT, _ := filepath.EvalSymlinks(wt)
	if lines[0] != realWT {
		t.Errorf("folder %q, want %q", lines[0], realWT)
	}
	if !strings.Contains(got, "# lazydev unblock: "+blockedPRDPath+"\n") || slices.Contains(lines, "-p") {
		t.Errorf("arguments:\n%s", got)
	}
	if i := slices.Index(lines, "--agents"); i < 0 || i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], `{"committer":`) {
		t.Errorf("agents:\n%s", got)
	}
	// repo.Init added the prompt of the dialog.
	if _, err := os.Stat(filepath.Join(p.path, ".lazydev", "unblock.md")); err != nil {
		t.Error(err)
	}
}

func TestUnblockFromRunList(t *testing.T) {
	m, p, _ := blockedModel(t)
	fakeClaude(t)
	m.focus = colBottom
	m = press(t, m, keyRunes("f"))
	hangupAll(t, p)
	if term := p.activeTerminal(); m.err != "" || term == nil || term.TabTitle() != "unblock" {
		t.Errorf("err %q, tab %v", m.err, term)
	}
}

func TestUnblockNotBlocked(t *testing.T) {
	m, p, _ := blockedModel(t)
	p.runs["token"].Phase = project.PhaseError
	m = press(t, m, keyRunes("f"))
	if m.err != "auth/token is not blocked." || len(p.tabs) != 0 {
		t.Errorf("err %q, tabs %d", m.err, len(p.tabs))
	}
	delete(p.runs, "token")
	m = press(t, m, keyRunes("f"))
	if m.err != "auth/token is not blocked." {
		t.Errorf("without run: %q", m.err)
	}
}

func TestStartBlocked(t *testing.T) {
	m, _, wt := blockedModel(t)
	m = press(t, m, keyRunes("s"))
	if m.err != "auth/token is blocked. Resolve it with f first." || m.question != nil {
		t.Errorf("err %q, question %v", m.err, m.question)
	}
	// After the dialog the PRD is planned again and s asks as usual.
	resolved, _ := project.SetStatus(project.RemoveBlocker(blockedPRD), project.ValuePlanned)
	if err := os.WriteFile(filepath.Join(wt, blockedPRDPath), []byte(resolved), 0o644); err != nil {
		t.Fatal(err)
	}
	m = press(t, m, keyRunes("s"))
	if m.err != "" || m.question == nil || m.question.text != "Continue auth/token? (y/n)" {
		t.Errorf("err %q, question %+v", m.err, m.question)
	}
}

func TestBlockedFooterSymbolColor(t *testing.T) {
	m, p, _ := blockedModel(t)
	if keys := m.footerKeys(); !strings.Contains(keys, "D dev · f unblock · c status") {
		t.Errorf("PRDs footer: %q", keys)
	}
	m.focus = colBottom
	if keys := m.footerKeys(); !strings.Contains(keys, "D dev · f unblock · i edit") {
		t.Errorf("runs footer: %q", keys)
	}
	p.runs["token"].Phase = project.PhaseError
	for _, focus := range []int{colRight, colBottom} {
		m.focus = focus
		if keys := m.footerKeys(); strings.Contains(keys, "unblock") {
			t.Errorf("footer without blocked run: %q", keys)
		}
	}
	p.runs["token"].Phase = project.PhaseBlocked

	if c := phaseColor(project.PhaseBlocked, themes[0]); c != colorBlocked || colorBlocked != "#d7af00" {
		t.Errorf("color %v", c)
	}
	m.focus = colRight
	view := m.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"⊘ auth/token", "⊘ blocked", "implement blocked"} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	checkSize(t, view, 160, 40)
}
