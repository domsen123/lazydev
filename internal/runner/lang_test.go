package runner

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

// germanConfig is hooksConfig of a project with language: de.
const germanConfig = "language: de\n" + hooksConfig

// TestRunGerman runs a German project from plan to merge: German contracts, agents, status values
// and fields, exactly as before English became the default.
func TestRunGerman(t *testing.T) {
	target := newProject(t, germanConfig, true)
	write(t, target.Dir, prdPath, prdContentDE)
	t.Setenv("FAKE_CLAUDE_LANG", "de")
	saveOtherRun(t, target, "alt", project.PhaseWaiting)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged || r.AfterReason != "beide ändern die Tokens" || len(r.After) != 0 {
		t.Fatalf("state: %+v", r)
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(got, "> Status: umgesetzt (2026-10-20) · Kosten: 0.85 USD · Modell: claude-opus-5-5 · Scope: tokens\n") {
		t.Errorf("PRD on main:\n%s", got)
	}
	args := fakeArgs(t)
	for _, s := range []string{"## Regeln von lazydev", "## Pflichtlektüre", "Lies diese Dateien vollständig", "## Reihenfolge",
		"--agents\n" + filepath.Join(state.Dir(), "agents.de.json") + "\n"} {
		if !strings.Contains(args, s) {
			t.Errorf("claude args lack %q", s)
		}
	}
	for _, s := range []string{"## Rules of lazydev", "## Required reading", "agents.json\n"} {
		if strings.Contains(args, s) {
			t.Errorf("claude args of a German project contain %q", s)
		}
	}
}

// TestRunEnglishWithGermanAnswers runs an English project whose PRD has a German status line and
// whose Claude answers in German: reading both languages lets it run through, lazydev writes
// English.
func TestRunEnglishWithGermanAnswers(t *testing.T) {
	target := newProject(t, "language: en\n", false)
	write(t, target.Dir, prdPath, prdContentDE)
	t.Setenv("FAKE_CLAUDE_LANG", "de")
	saveOtherRun(t, target, "alt", project.PhaseWaiting)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged {
		t.Fatalf("state: %+v", r)
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(got, "> Status: umgesetzt (2026-10-20) · Cost: 0.85 USD · Model: claude-opus-5-5 · Scope: tokens\n") {
		t.Errorf("PRD on main:\n%s", got)
	}
	if args := fakeArgs(t); !strings.Contains(args, "## Rules of lazydev") || strings.Contains(args, "## Regeln von lazydev") {
		t.Errorf("an English project got German contracts:\n%s", args)
	}
}

// TestBlockerLanguages checks the blocker in both languages and that the run continues after the
// dialog resolved it.
func TestBlockerLanguages(t *testing.T) {
	for _, tt := range []struct {
		name, config, claude, prd string
		blocked, step, value      string
	}{
		{"english", "", "", prdContent, "blocked", "Step:", "planned"},
		{"german", "language: de\n", "de", prdContentDE, "blockiert", "Schritt:", "geplant"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target := newProject(t, tt.config, false)
			write(t, target.Dir, prdPath, tt.prd)
			t.Setenv("FAKE_CLAUDE_LANG", tt.claude)
			t.Setenv("FAKE_CLAUDE_MODE", "abort")
			if err := Run(context.Background(), target, prdName); err == nil || err.Error() != "implement blocked: Feld fehlt im PRD" {
				t.Fatalf("Run: %v", err)
			}
			wt := WorktreePath("app", prdName)
			want := "# PRD: Token\n\n> Status: " + tt.blocked + " (" + today() + ") · Scope: tokens\n\nbody\n\n" +
				"## Blocker\n\n" + tt.step + " implement\n\n- Feld fehlt im PRD\n- Zweite Frage\n"
			if got := read(t, filepath.Join(wt, prdPath)); got != want {
				t.Errorf("PRD in the worktree:\n%s", got)
			}

			if err := Run(context.Background(), target, prdName); err == nil || !strings.Contains(err.Error(), "still blocked") {
				t.Fatalf("Run while blocked: %v", err)
			}
			resolveBlocker(t, tt.value)
			t.Setenv("FAKE_CLAUDE_MODE", "ok")
			if err := Run(context.Background(), target, prdName); err != nil {
				t.Fatal(err)
			}
			prd := read(t, filepath.Join(target.Dir, prdPath))
			if r := loadRun(t); r.Phase != project.PhaseMerged || strings.Contains(prd, "## Blocker") {
				t.Errorf("state %s, PRD:\n%s", r.Phase, prd)
			}
		})
	}
}

// TestAutoUnblockLanguages checks that resolve gets its contract in the language of the project
// and that its decisions land in the PRD.
func TestAutoUnblockLanguages(t *testing.T) {
	for _, tt := range []struct {
		name, config, claude, prd, contract, decisions string
	}{
		{"english", "", "", prdContent, "# Resolve a blocker yourself", "## Decisions"},
		{"german", "language: de\n", "de", prdContentDE, "# Blocker selbst klären", "## Entscheidungen"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target := newProject(t, tt.config, false)
			write(t, target.Dir, prdPath, tt.prd)
			autoUnblock(t, target)
			t.Setenv("FAKE_CLAUDE_LANG", tt.claude)
			t.Setenv("FAKE_CLAUDE_MODE", "abortonce")
			if err := Run(context.Background(), target, prdName); err != nil {
				t.Fatal(err)
			}
			r := loadRun(t)
			if r.Phase != project.PhaseMerged || r.Resolves() != 1 {
				t.Fatalf("state: %+v", r)
			}
			if prd := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(prd, tt.decisions) || strings.Contains(prd, "## Blocker") {
				t.Errorf("PRD:\n%s", prd)
			}
			if args := fakeArgs(t); !strings.Contains(args, tt.contract) {
				t.Errorf("resolve prompt lacks %q", tt.contract)
			}
			if log := read(t, state.LogPath("app", prdName, state.StepResolve)); !strings.Contains(log, "Decision: Feld fehlt im PRD: A, passt zum Code") {
				t.Errorf("resolve log:\n%s", log)
			}
		})
	}
}

// TestRecordRunInfoLanguages checks that the merge writes Cost and Model in English and Kosten
// and Modell in German.
func TestRecordRunInfoLanguages(t *testing.T) {
	for _, tt := range []struct {
		config, want, not string
	}{
		{"", "Cost: 0.80 USD · Model: claude-opus-5-5", "Kosten:"},
		{"language: de\n", "Kosten: 0.80 USD · Modell: claude-opus-5-5", "Cost:"},
	} {
		target := newProject(t, tt.config, false)
		if err := Run(context.Background(), target, prdName); err != nil {
			t.Fatal(err)
		}
		got := read(t, filepath.Join(target.Dir, prdPath))
		if !strings.Contains(got, tt.want) || strings.Contains(got, tt.not) {
			t.Errorf("%q: PRD on main:\n%s", tt.config, got)
		}
	}
}

// TestDiscardRestoresPlannedInLanguage checks that X puts the PRD back with the planned value of
// the project.
func TestDiscardRestoresPlannedInLanguage(t *testing.T) {
	for _, tt := range []struct{ config, want string }{{"", "> Status: planned · Scope: tokens"}, {"language: de\n", "> Status: geplant · Scope: tokens"}} {
		target := newProject(t, tt.config, false)
		t.Setenv("FAKE_CLAUDE_MODE", "abort")
		if err := Run(context.Background(), target, prdName); err == nil {
			t.Fatal("Run did not block")
		}
		if err := Discard(target, prdName); err != nil {
			t.Fatal(err)
		}
		if got := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(got, tt.want) || strings.Contains(got, "## Blocker") {
			t.Errorf("%q: PRD:\n%s", tt.config, got)
		}
	}
}

func TestDecisionsHeading(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "de.md", "# PRD: X\n\n## Entscheidungen\n\n- a\n")
	write(t, dir, "en.md", "# PRD: X\n\n## Decisions\n\n- a\n")
	for file, want := range map[string]string{"de.md": "## Entscheidungen", "en.md": "## Decisions", "missing.md": "## Decisions"} {
		if got := decisionsHeading(filepath.Join(dir, file)); got != want {
			t.Errorf("decisionsHeading(%s) = %q", file, got)
		}
	}
}

func TestEvidenceAndPointsBothLanguages(t *testing.T) {
	if got := reportPoints("- none\n- keine\n- a\n"); !slices.Equal(got, []string{"a"}) {
		t.Errorf("reportPoints = %q", got)
	}
	if got := evidenceFiles("- none\n- a.png: x\n", "/e"); !slices.Equal(got, []string{"a.png"}) {
		t.Errorf("evidenceFiles = %q", got)
	}
}
