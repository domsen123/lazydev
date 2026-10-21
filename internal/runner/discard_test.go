package runner

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/fsutil"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestDiscardAfterFailedRun(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	failAtReview(t, target)
	wt := WorktreePath("app", prdName)
	// The implementation set the status in the worktree.
	if got := read(t, filepath.Join(wt, prdPath)); !strings.Contains(got, "> Status: umgesetzt (2026-10-20)") {
		t.Fatalf("PRD in the worktree:\n%s", got)
	}
	if err := Discard(target, prdName); err != nil {
		t.Fatal(err)
	}
	if fsutil.Exists(wt) {
		t.Error("worktree still exists")
	}
	if got := runGit(t, target.Dir, "branch", "--list", "feat/new-token"); got != "" {
		t.Errorf("branch still exists: %q", got)
	}
	if _, err := state.Load("app", prdName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("state: %v", err)
	}
	for _, step := range state.Steps {
		if fsutil.Exists(state.LogPath("app", prdName, step)) {
			t.Errorf("log of %s still exists", step)
		}
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); got != prdContent {
		t.Errorf("PRD in the main folder:\n%s", got)
	}
	if got := runGit(t, target.Dir, "status", "--porcelain", "--untracked-files=all"); got != "?? "+prdPath {
		t.Errorf("main: %q", got)
	}
	if got := read(t, filepath.Join(target.Dir, "hooks.log")); !strings.Contains(got, "teardown ") {
		t.Errorf("teardown did not run:\n%s", got)
	}

	// The PRD starts over.
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged || len(r.Steps) != 4 {
		t.Errorf("state after the new run: %+v", r)
	}
}

func TestDiscardKeepsExistingPRD(t *testing.T) {
	target := newProject(t, "", false)
	t.Setenv("FAKE_CLAUDE_MODE", "abort")
	if err := Run(context.Background(), target, prdName); err == nil {
		t.Fatal("Run without error")
	}
	// A PRD that is back in the main folder stays as it is.
	write(t, target.Dir, prdPath, "# PRD: Token\n\n> Status: geplant · Scope: edited\n")
	if err := Discard(target, prdName); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(got, "Scope: edited") {
		t.Errorf("PRD overwritten:\n%s", got)
	}
}

func TestDiscardWithoutState(t *testing.T) {
	target := newProject(t, "", false)
	if err := Discard(target, prdName); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); got != prdContent {
		t.Errorf("PRD changed:\n%s", got)
	}

	// A half created run without state: worktree and branch exist.
	wt := WorktreePath("app", prdName)
	runGit(t, target.Dir, "worktree", "add", "-q", "-b", "feat/new-token", wt, "main")
	write(t, wt, "wip.txt", "x\n")
	if err := Discard(target, prdName); err != nil {
		t.Fatal(err)
	}
	if fsutil.Exists(wt) || runGit(t, target.Dir, "branch", "--list", "feat/new-token") != "" {
		t.Error("worktree or branch left")
	}
}

func TestDiscardSkipsUntrustedHook(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	failAtReview(t, target)
	// A pulled config.yaml with a changed teardown hook is not trusted yet.
	write(t, target.Dir, ".lazydev/config.yaml", strings.Replace(hooksConfig, "teardown: teardown.sh", "teardown: ./teardown.sh", 1))
	err := Discard(target, prdName)
	if err == nil || !strings.Contains(err.Error(), "teardown hook not run") {
		t.Fatalf("Discard: %v", err)
	}
	if got := read(t, filepath.Join(target.Dir, "hooks.log")); strings.Contains(got, "teardown ") {
		t.Errorf("untrusted teardown ran:\n%s", got)
	}
	if fsutil.Exists(WorktreePath("app", prdName)) {
		t.Error("worktree still exists")
	}
	if _, err := state.Load("app", prdName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("state: %v", err)
	}
}
