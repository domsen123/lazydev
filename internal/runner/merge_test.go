package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/fsutil"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// failAtReview runs until the review fails on a dirty worktree, so the test can change main
// before the merge.
func failAtReview(t *testing.T, target Target) {
	t.Helper()
	t.Setenv("FAKE_CLAUDE_MODE", "dirty")
	if err := Run(context.Background(), target, prdName); err == nil || !strings.Contains(err.Error(), "worktree after the review not clean") {
		t.Fatalf("Run with a dirty review: %v", err)
	}
	t.Setenv("FAKE_CLAUDE_MODE", "ok")
}

// commitOnMain commits review.txt on main with other content than the review of the branch.
func commitOnMain(t *testing.T, target Target, file string) {
	t.Helper()
	write(t, target.Dir, file, "main\n")
	runGit(t, target.Dir, "add", file)
	runGit(t, target.Dir, "commit", "-q", "-m", "main: "+file)
}

func TestMergeRebasesWithoutConflict(t *testing.T) {
	target := newProject(t, "", false)
	failAtReview(t, target)
	commitOnMain(t, target, "other.txt")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, target.Dir, "log", "--format=%s"); got != "docs(prd): record cost of new-token\nfeat: review\ndocs(prd): add new-token\nmain: other.txt\ninit" {
		t.Errorf("main: %q", got)
	}
	if strings.Contains(fakeArgs(t), "# lazydev merge:") {
		t.Error("Claude step merge ran without conflict")
	}
}

func TestMergeConflictRunsClaude(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	failAtReview(t, target)
	commitOnMain(t, target, "review.txt")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged || r.Note != "" {
		t.Errorf("state: %+v", r)
	}
	last := r.LastStep()
	if last.Name != state.StepMerge || !last.OK || last.SessionID != "sess-1" || last.CostUSD == nil || *last.CostUSD != 0.1 {
		t.Errorf("merge step: %+v", last)
	}
	main := target.Dir
	if got := runGit(t, main, "log", "--format=%s"); got != "docs(prd): record cost of new-token\nfeat: review\ndocs(prd): add new-token\nmain: review.txt\ninit" {
		t.Errorf("main: %q", got)
	}
	// The branch wins in the fake's rebase.
	if got := read(t, filepath.Join(main, "review.txt")); got != "change\n" {
		t.Errorf("review.txt: %q", got)
	}
	// Implement, the failed and the second review and the Claude step merge add up.
	if got := read(t, filepath.Join(main, prdPath)); !strings.Contains(got, "Kosten: 1.20 USD · Modell: claude-opus-5-5") {
		t.Errorf("PRD on main:\n%s", got)
	}
	log := read(t, state.LogPath("app", prdName, state.StepMerge))
	for _, s := range []string{"Conflict in the rebase", "# lazydev merge: feat/new-token", "Merged"} {
		if !strings.Contains(log, s) {
			t.Errorf("merge log lacks %q:\n%s", s, log)
		}
	}
	if args := fakeArgs(t); !strings.Contains(args, "-p\n# lazydev merge: feat/new-token\n") {
		t.Errorf("claude args:\n%s", args)
	}
	if got := runGit(t, main, "status", "--porcelain"); got != "" {
		t.Errorf("main not clean: %q", got)
	}
}

func TestMergeClaudeBlocked(t *testing.T) {
	target := newProject(t, "", false)
	failAtReview(t, target)
	commitOnMain(t, target, "review.txt")
	t.Setenv("FAKE_MERGE_MODE", "abort")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "merge blocked: Konflikt in review.txt" {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseBlocked || r.Error == nil || *r.Error != err.Error() || r.PID != 0 || r.NextStep() != state.StepMerge {
		t.Errorf("state: %+v", r)
	}
	if last := r.LastStep(); last.OK || *last.ExitCode != 0 {
		t.Errorf("merge step: %+v", last)
	}
	// The rebase was aborted, the worktree is clean on the branch with the blocker committed.
	wt := WorktreePath("app", prdName)
	if got := runGit(t, wt, "status", "--porcelain"); got != "" {
		t.Errorf("worktree: %q", got)
	}
	if got := runGit(t, wt, "branch", "--show-current"); got != "feat/new-token" {
		t.Errorf("worktree on %q", got)
	}
	if got := runGit(t, wt, "log", "-1", "--format=%B"); got != "docs(prd): record blocker of new-token" {
		t.Errorf("last commit: %q", got)
	}
	prd := read(t, filepath.Join(wt, prdPath))
	if !strings.Contains(prd, "> Status: blockiert ("+today()+") · Scope: tokens") ||
		!strings.HasSuffix(prd, "## Blocker\n\nSchritt: merge\n\n- Konflikt in review.txt\n") {
		t.Errorf("PRD in the worktree:\n%s", prd)
	}
	if got := runGit(t, target.Dir, "log", "-1", "--format=%s"); got != "main: review.txt" {
		t.Errorf("main moved: %q", got)
	}

	resolveBlocker(t, "umgesetzt ("+today()+")")
	t.Setenv("FAKE_MERGE_MODE", "ok")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged {
		t.Errorf("phase: %s", r.Phase)
	}
	got := runGit(t, target.Dir, "log", "--format=%s")
	if !strings.Contains(got, "docs(prd): record cost of new-token\ndocs(prd): resolve blocker of new-token\ndocs(prd): record blocker of new-token\n") {
		t.Errorf("main: %q", got)
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); strings.Contains(got, "## Blocker") || !strings.Contains(got, "> Status: umgesetzt ("+today()+")") {
		t.Errorf("PRD on main:\n%s", got)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepMerge)); !strings.Contains(log, "Commit: docs(prd): resolve blocker of new-token") {
		t.Errorf("merge log:\n%s", log)
	}
}

func TestMergeClaudeLeavesBranchOffBase(t *testing.T) {
	target := newProject(t, "", false)
	failAtReview(t, target)
	commitOnMain(t, target, "review.txt")
	t.Setenv("FAKE_MERGE_MODE", "noop")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "after the Claude step merge, feat/new-token is not on main" {
		t.Fatalf("Run: %v", err)
	}
}

func TestMergeCheckRedOnceRunsClaude(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	t.Setenv("FAKE_CHECK", "once")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged {
		t.Errorf("phase: %s", r.Phase)
	}
	args := fakeArgs(t)
	if !strings.Contains(args, "-p\n# lazydev merge: feat/new-token Prüfung nach dem Rebase fehlgeschlagen: check hook ended with exit code 1\n") {
		t.Errorf("claude args:\n%s", args)
	}
	if got := strings.Count(read(t, filepath.Join(target.Dir, "hooks.log")), "check "); got != 2 {
		t.Errorf("check ran %d times", got)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepMerge)); !strings.Contains(log, "Check red: check hook ended with exit code 1") {
		t.Errorf("merge log:\n%s", log)
	}
}

func TestMergeCheckStaysRed(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	t.Setenv("FAKE_CHECK", "fail")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "check still red after the Claude step merge: check hook ended with exit code 1" {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseError || r.NextStep() != state.StepMerge {
		t.Errorf("state: %+v", r)
	}
	if !fsutil.Exists(WorktreePath("app", prdName)) {
		t.Error("worktree removed")
	}
	if got := runGit(t, target.Dir, "log", "-1", "--format=%s"); got != "init" {
		t.Errorf("main moved: %q", got)
	}
}

func TestMergeWaitsForCleanMainFolder(t *testing.T) {
	target := newProject(t, "", false)
	write(t, target.Dir, "CLAUDE.md", "# app\nlocal change\n")
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), target, prdName) }()
	waitFor(t, "the merge queue", func() bool {
		r, err := state.Load("app", prdName)
		return err == nil && r.Phase == project.PhaseMergeQueue && r.Note == "main folder: changed: CLAUDE.md"
	})
	runGit(t, target.Dir, "checkout", "--", "CLAUDE.md")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged || r.Note != "" {
		t.Errorf("state: %+v", r)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepMerge)); !strings.Contains(log, "Main folder blocks the merge: changed: CLAUDE.md") {
		t.Errorf("merge log:\n%s", log)
	}
}

func TestMergeAbortedInQueueContinues(t *testing.T) {
	target := newProject(t, "", false)
	runGit(t, target.Dir, "switch", "-q", "-c", "other")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, target, prdName) }()
	waitFor(t, "the merge queue", func() bool {
		r, err := state.Load("app", prdName)
		return err == nil && r.Note == "main folder: on other instead of main"
	})
	cancel()
	if err := <-done; err == nil || err.Error() != "merge aborted" {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseAborted || r.PID != 0 || r.NextStep() != state.StepMerge {
		t.Errorf("state: %+v", r)
	}

	runGit(t, target.Dir, "switch", "-q", "main")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged || r.Note != "" {
		t.Errorf("state: %+v", r)
	}
	// The cost commit of the first attempt stays, the second attempt adds none.
	if got := runGit(t, target.Dir, "log", "--format=%s"); got != "docs(prd): record cost of new-token\nfeat: review\ndocs(prd): add new-token\ninit" {
		t.Errorf("main: %q", got)
	}
}

func TestMergeWaitsForMergeLock(t *testing.T) {
	target := newProject(t, "", false)
	unlock, err := state.Lock(state.MergeLock("app"))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), target, prdName) }()
	waitFor(t, "the merge slot", func() bool {
		data, _ := os.ReadFile(state.LogPath("app", prdName, state.StepMerge))
		return strings.Contains(string(data), "Another PRD merges, waiting for the merge slot")
	})
	if r := loadRun(t); r.Phase != project.PhaseMergeQueue {
		t.Errorf("phase while waiting: %s", r.Phase)
	}
	unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged {
		t.Errorf("phase: %s", r.Phase)
	}
}

func TestMainDirProblem(t *testing.T) {
	target := newProject(t, "", false)
	dir := target.Dir
	runGit(t, dir, "add", prdPath)
	runGit(t, dir, "commit", "-q", "-m", "prd")
	check := func(want string) {
		t.Helper()
		got, err := mainDirProblem(dir, "main")
		if err != nil || got != want {
			t.Errorf("mainDirProblem = %q, %v, want %q", got, err, want)
		}
	}
	check("")
	// Untracked PRDs and anything under .lazydev/ do not block.
	write(t, dir, "prd/other/new.md", "# PRD: New\n")
	write(t, dir, ".lazydev/local.md", "x\n")
	write(t, dir, ".lazydev/implement.md", "changed\n")
	check("")
	write(t, dir, "notes.txt", "x\n")
	check("changed: notes.txt")
	runGit(t, dir, "clean", "-q", "-f", "notes.txt")
	// A tracked PRD counts.
	write(t, dir, prdPath, "changed\n")
	check("changed: " + prdPath)
	runGit(t, dir, "checkout", "--", prdPath)
	check("")
	runGit(t, dir, "switch", "-q", "-c", "other")
	check("on other instead of main")
	runGit(t, dir, "switch", "-q", "--detach", "main")
	check("on HEAD instead of main")
	if _, err := mainDirProblem(t.TempDir(), "main"); err == nil {
		t.Error("no error outside a repository")
	}
}

// After the main folder got clean, the merge goes on in phase merge: merge-queue is no active
// phase, so a runner that dies in the fast-forward or cleanup would not show as aborted.
func TestMergePhaseAfterMainFolderWait(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	// The teardown runs after the fast-forward and records the phase of the state.
	write(t, target.Dir, "teardown.sh", `#!/bin/sh
grep -o '"phase": "[a-z-]*"' "$XDG_STATE_HOME/lazydev/runs/app/new-token.json" >> "$LAZYDEV_PROJECT/hooks.log"
`)
	runGit(t, target.Dir, "commit", "-q", "-am", "teardown records the phase")
	trustProject(t, target)
	write(t, target.Dir, "CLAUDE.md", "# app\nlocal change\n")
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), target, prdName) }()
	waitFor(t, "the merge queue", func() bool {
		r, err := state.Load("app", prdName)
		return err == nil && r.Note != ""
	})
	runGit(t, target.Dir, "checkout", "--", "CLAUDE.md")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(target.Dir, "hooks.log")); !strings.Contains(got, `"phase": "merge"`+"\n") {
		t.Errorf("phase during the teardown:\n%s", got)
	}
}
