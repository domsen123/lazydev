package runner

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// autoUnblock switches auto-unblock on for target.
func autoUnblock(t *testing.T, target Target) {
	t.Helper()
	s := config.LoadSettings()
	s.SetAutoUnblock(target.Dir, true)
	if err := config.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
}

func stepNames(r *state.Run) string {
	var names []string
	for _, s := range r.Steps {
		names = append(names, s.Name)
	}
	return strings.Join(names, " ")
}

// TestAutoUnblockResolves checks that a blocked implement gets resolved, the PRD committed alone
// and implement runs again up to the merge.
func TestAutoUnblockResolves(t *testing.T) {
	target := newProject(t, "", false)
	autoUnblock(t, target)
	t.Setenv("FAKE_CLAUDE_MODE", "abortonce")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged || r.Error != nil {
		t.Fatalf("state: %+v", r)
	}
	if got := stepNames(r); got != "plan setup implement resolve implement review merge" {
		t.Errorf("steps: %s", got)
	}
	if r.Resolves() != 1 || !r.Steps[3].OK || r.Steps[3].CostUSD == nil || *r.Steps[3].CostUSD != 0.1 {
		t.Errorf("resolve step: %+v", r.Steps[3])
	}
	logs := strings.Join(git.Output(target.Dir, "log", "--format=%s"), "\n")
	if !strings.Contains(logs, "docs(prd): resolve blocker of new-token") {
		t.Errorf("git log:\n%s", logs)
	}
	prd := read(t, filepath.Join(target.Dir, prdPath))
	if !strings.Contains(prd, "## Entscheidungen") || strings.Contains(prd, "## Blocker") {
		t.Errorf("PRD:\n%s", prd)
	}
	log := read(t, state.LogPath("app", prdName, state.StepResolve))
	for _, s := range []string{"Claude resolves it", "Decision: Feld fehlt im PRD: A, passt zum Code", "Resolved, implement runs again"} {
		if !strings.Contains(log+read(t, state.LogPath("app", prdName, state.StepImplement)), s) {
			t.Errorf("logs lack %q", s)
		}
	}
}

// TestAutoUnblockDeclines checks that a resolve that may not decide leaves the run blocked with the
// PRD as the block committed it.
func TestAutoUnblockDeclines(t *testing.T) {
	for _, mode := range []string{"decline", "dirty"} {
		t.Run(mode, func(t *testing.T) {
			target := newProject(t, "", false)
			autoUnblock(t, target)
			t.Setenv("FAKE_CLAUDE_MODE", "abort")
			t.Setenv("FAKE_RESOLVE", mode)
			err := Run(context.Background(), target, prdName)
			var blocked *blockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("Run: %v", err)
			}
			r := loadRun(t)
			if r.Phase != project.PhaseBlocked || r.PID != 0 || r.NextStep() != state.StepImplement || r.Error == nil || !strings.HasPrefix(*r.Error, "implement blocked:") {
				t.Errorf("state: %+v", r)
			}
			if got := stepNames(r); got != "plan setup implement resolve" || r.Steps[3].OK {
				t.Errorf("steps: %s", got)
			}
			wt := WorktreePath("app", prdName)
			prd := read(t, filepath.Join(wt, prdPath))
			if !strings.Contains(prd, "## Blocker") || strings.Contains(prd, "kaputt") || !project.BlockedIn(wt, prdPath) {
				t.Errorf("PRD in the worktree:\n%s", prd)
			}
			want := map[string]string{"decline": "Claude may not decide: Datenverlust möglich", "dirty": "resolve changed other.txt"}[mode]
			if log := read(t, state.LogPath("app", prdName, state.StepResolve)); !strings.Contains(log, want) {
				t.Errorf("resolve log lacks %q:\n%s", want, log)
			}
		})
	}
}

// TestAutoUnblockLimit checks that a run resolves at most MaxResolves times.
func TestAutoUnblockLimit(t *testing.T) {
	target := newProject(t, "", false)
	autoUnblock(t, target)
	t.Setenv("FAKE_CLAUDE_MODE", "abort")
	err := Run(context.Background(), target, prdName)
	var blocked *blockedError
	if !errors.As(err, &blocked) || blocked.resolve {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseBlocked || r.Resolves() != MaxResolves {
		t.Errorf("state: %+v", r)
	}
	if got := stepNames(r); got != "plan setup implement resolve implement resolve implement" {
		t.Errorf("steps: %s", got)
	}
}

// TestAutoUnblockOff checks that without the setting a block stops the run right away.
func TestAutoUnblockOff(t *testing.T) {
	target := newProject(t, "", false)
	t.Setenv("FAKE_CLAUDE_MODE", "abort")
	if err := Run(context.Background(), target, prdName); err == nil {
		t.Fatal("Run without error")
	}
	if got := stepNames(loadRun(t)); got != "plan setup implement" {
		t.Errorf("steps: %s", got)
	}
}
