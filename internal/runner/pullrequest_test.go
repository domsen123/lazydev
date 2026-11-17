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

// prConfig switches the project to pull requests.
const prConfig = "merge: pr\n"

const firstPR = "https://github.com/o/r/pull/1"

// newPRProject is a project with merge: pr and a bare repository as origin.
func newPRProject(t *testing.T) (Target, string) {
	t.Helper()
	target := newProject(t, prConfig, false)
	return target, withOrigin(t, target)
}

// runUntilPR runs the PRD in the background until it waits in phase pr and returns the channel of
// its end.
func runUntilPR(t *testing.T, ctx context.Context, target Target) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, target, prdName) }()
	waitFor(t, "phase pr", func() bool {
		select {
		case err := <-done:
			t.Fatalf("Run ended before phase pr: %v", err)
		default:
		}
		r, err := state.Load("app", prdName)
		return err == nil && r.Phase == project.PhasePR
	})
	return done
}

// mergeOnGitHub imitates the merge of the pull request: origin's main gets the branch and the fake
// gh reports MERGED.
func mergeOnGitHub(t *testing.T, target Target) {
	t.Helper()
	runGit(t, target.Dir, "push", "-q", "origin", "feat/new-token:main")
	setGHFile(t, "state", "MERGED")
}

func TestPullRequestMerged(t *testing.T) {
	target, origin := newPRProject(t)
	done := runUntilPR(t, context.Background(), target)

	r := loadRun(t)
	if r.PullRequest != firstPR || r.Note != "" {
		t.Errorf("state in phase pr: %+v", r)
	}
	// The branch is on origin, rebased onto origin/main, with the cost commit.
	wt := WorktreePath("app", prdName)
	if got, want := runGit(t, origin, "rev-parse", "feat/new-token"), runGit(t, wt, "rev-parse", "HEAD"); got != want {
		t.Errorf("origin has %s, the branch %s", got, want)
	}
	if got := runGit(t, wt, "log", "-1", "--format=%s"); got != "docs(prd): record cost of new-token" {
		t.Errorf("last commit: %q", got)
	}
	// Title and description come from pr.md of the review.
	if got := ghFile(t, "title"); got != "feat(auth): add tokens" {
		t.Errorf("title: %q", got)
	}
	if got := ghFile(t, "body"); got != "## Summary\n\nTokens." {
		t.Errorf("body: %q", got)
	}
	if calls := ghFile(t, "calls"); !strings.Contains(calls, "pr create --base main --head feat/new-token --title feat(auth): add tokens --body-file ") {
		t.Errorf("gh calls:\n%s", calls)
	}
	// main stays where it was until the pull request is merged.
	if got := runGit(t, target.Dir, "log", "-1", "--format=%s"); got != "init" {
		t.Errorf("main moved: %q", got)
	}
	// The merge slot is free while the run waits.
	unlock, err := state.TryLock(state.MergeLock("app"))
	if err != nil || unlock == nil {
		t.Fatalf("merge lock held in phase pr: %v", err)
	}
	unlock()

	mergeOnGitHub(t, target)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r = loadRun(t)
	if r.Phase != project.PhaseMerged || r.Note != "" || r.PullRequest != firstPR {
		t.Errorf("state: %+v", r)
	}
	if fsutil.Exists(wt) {
		t.Error("worktree still exists")
	}
	if got := runGit(t, target.Dir, "branch", "--list", "feat/new-token"); got != "" {
		t.Errorf("branch still exists: %q", got)
	}
	// The main folder follows origin/main.
	if got, want := runGit(t, target.Dir, "rev-parse", "main"), runGit(t, origin, "rev-parse", "main"); got != want {
		t.Errorf("main at %s, origin/main at %s", got, want)
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(got, "Kosten: 0.80 USD") {
		t.Errorf("PRD on main:\n%s", got)
	}
	log := read(t, state.LogPath("app", prdName, state.StepMerge))
	for _, s := range []string{"$ git fetch origin main", "$ git rebase origin/main", "$ git push --force-with-lease -u origin feat/new-token",
		"Pull request " + firstPR + " created", "Pull request " + firstPR + " merged", "$ git merge --ff-only origin/main", "Merged"} {
		if !strings.Contains(log, s) {
			t.Errorf("merge log lacks %q:\n%s", s, log)
		}
	}
}

func TestPullRequestMainFolderNotUpdated(t *testing.T) {
	target, _ := newPRProject(t)
	done := runUntilPR(t, context.Background(), target)
	write(t, target.Dir, "CLAUDE.md", "# app\nlocal change\n")
	mergeOnGitHub(t, target)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged || r.Note != "main folder not updated: changed: CLAUDE.md" {
		t.Errorf("state: %+v", r)
	}
	// main stays, the change stays.
	if got := runGit(t, target.Dir, "log", "-1", "--format=%s"); got != "init" {
		t.Errorf("main moved: %q", got)
	}
	if fsutil.Exists(WorktreePath("app", prdName)) {
		t.Error("worktree still exists")
	}
}

func TestPullRequestClosedAndContinued(t *testing.T) {
	target, _ := newPRProject(t)
	done := runUntilPR(t, context.Background(), target)
	setGHFile(t, "state", "CLOSED")
	err := <-done
	want := "pull request " + firstPR + " closed without merge"
	if err == nil || err.Error() != want {
		t.Fatalf("Run: %v, want %s", err, want)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseError || r.NextStep() != state.StepMerge || !fsutil.Exists(r.Worktree) {
		t.Errorf("state: %+v", r)
	}

	// s rebases, checks, pushes and opens a new pull request.
	done = runUntilPR(t, context.Background(), target)
	if r := loadRun(t); r.PullRequest != "https://github.com/o/r/pull/2" {
		t.Errorf("pull request: %q", r.PullRequest)
	}
	if got := ghFile(t, "count"); got != "2" {
		t.Errorf("created %s pull requests", got)
	}
	mergeOnGitHub(t, target)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPullRequestReusesOpen(t *testing.T) {
	target, _ := newPRProject(t)
	// An open pull request of the branch exists already, e.g. from an attempt before an error.
	setGHFile(t, "url", "https://github.com/o/r/pull/9")
	setGHFile(t, "state", "OPEN")
	setGHFile(t, "head", "old")
	done := runUntilPR(t, context.Background(), target)
	if r := loadRun(t); r.PullRequest != "https://github.com/o/r/pull/9" {
		t.Errorf("pull request: %q", r.PullRequest)
	}
	calls := ghFile(t, "calls")
	if strings.Contains(calls, "pr create") || !strings.Contains(calls, "pr edit https://github.com/o/r/pull/9 --title feat(auth): add tokens --body-file ") {
		t.Errorf("gh calls:\n%s", calls)
	}
	mergeOnGitHub(t, target)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPullRequestResumesAfterAbort(t *testing.T) {
	target, _ := newPRProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := runUntilPR(t, ctx, target)
	cancel()
	if err := <-done; err == nil || err.Error() != "merge aborted" {
		t.Fatalf("Run: %v", err)
	}
	if r := loadRun(t); r.Phase != project.PhaseAborted || r.NextStep() != state.StepMerge {
		t.Errorf("state: %+v", r)
	}
	pushes := strings.Count(read(t, state.LogPath("app", prdName, state.StepMerge)), "$ git push")

	// The pull request is still the current one: s waits again without rebase and push.
	done = runUntilPR(t, context.Background(), target)
	mergeOnGitHub(t, target)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	log := read(t, state.LogPath("app", prdName, state.StepMerge))
	if got := strings.Count(log, "$ git push"); got != pushes {
		t.Errorf("pushed again after the abort:\n%s", log)
	}
	if !strings.Contains(log, "Pull request "+firstPR+" is current, waiting for it again") {
		t.Errorf("merge log:\n%s", log)
	}
	if got := ghFile(t, "count"); got != "1" {
		t.Errorf("created %s pull requests", got)
	}
}

func TestPullRequestStateNotReadable(t *testing.T) {
	target, _ := newPRProject(t)
	done := runUntilPR(t, context.Background(), target)
	setGHFile(t, "viewfail", "")
	waitFor(t, "the note", func() bool {
		r, err := state.Load("app", prdName)
		return err == nil && r.Note == "pull request state not readable"
	})
	if r := loadRun(t); r.Phase != project.PhasePR {
		t.Errorf("phase: %s", r.Phase)
	}
	if err := os.Remove(filepath.Join(os.Getenv("FAKE_GH"), "viewfail")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the note to go", func() bool {
		r, err := state.Load("app", prdName)
		return err == nil && r.Note == ""
	})
	mergeOnGitHub(t, target)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	log := read(t, state.LogPath("app", prdName, state.StepMerge))
	if strings.Count(log, "PR state not readable: gh pr view: HTTP 502: bad gateway") != 1 || !strings.Contains(log, "PR state readable again") {
		t.Errorf("merge log:\n%s", log)
	}
}

func TestPullRequestTakesNoSlot(t *testing.T) {
	isolate(t)
	pid := startFakeRunner(t)
	entries := []state.Entry{{ID: "app", PRDName: "a", Run: &state.Run{PID: pid, Phase: project.PhasePR}}}
	if n := countRunning(entries); n != 0 {
		t.Errorf("countRunning with a runner in phase pr = %d", n)
	}
	entries[0].Run.Phase = project.PhaseMerge
	if n := countRunning(entries); n != 1 {
		t.Errorf("countRunning with a runner in phase merge = %d", n)
	}
}

func TestDiscardClosesPullRequest(t *testing.T) {
	target, _ := newPRProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := runUntilPR(t, ctx, target)
	cancel()
	<-done
	if err := Discard(target, prdName); err != nil {
		t.Fatal(err)
	}
	if got := ghFile(t, "state"); got != "CLOSED" {
		t.Errorf("pull request state: %s", got)
	}
	if calls := ghFile(t, "calls"); !strings.Contains(calls, "pr close "+firstPR+" --delete-branch") {
		t.Errorf("gh calls:\n%s", calls)
	}
	if fsutil.Exists(state.EvidenceDir("app", prdName)) {
		t.Error("evidence folder still exists")
	}
}

func TestPullRequestWithoutGH(t *testing.T) {
	target, _ := newPRProject(t)
	GH = []string{filepath.Join(t.TempDir(), "gh")}
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "gh not found, install the GitHub CLI for merge: pr" {
		t.Fatalf("Run: %v", err)
	}
	if r := loadRun(t); r.Phase != project.PhaseError || r.NextStep() != state.StepMerge {
		t.Errorf("state: %+v", r)
	}
}

func TestPullRequestNotAuthenticated(t *testing.T) {
	target, _ := newPRProject(t)
	setGHFile(t, "noauth", "")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "gh is not authenticated, run gh auth login" {
		t.Fatalf("Run: %v", err)
	}
}

func TestReviewWithoutPullRequestDescription(t *testing.T) {
	target, _ := newPRProject(t)
	t.Setenv("FAKE_REVIEW_PR", "none")
	err := Run(context.Background(), target, prdName)
	want := "review wrote no pull request description to " + filepath.Join(state.EvidenceDir("app", prdName), "pr.md")
	if err == nil || err.Error() != want {
		t.Fatalf("Run: %v, want %s", err, want)
	}
}

func TestPullRequestTextWithoutFile(t *testing.T) {
	target := newProject(t, prConfig, false)
	wt := t.TempDir()
	write(t, wt, prdPath, "# PRD: Token für Apps\n\n> Status: umgesetzt (2026-10-20) · Kosten: 0.80 USD · Modell: m · Scope: tokens\n")
	r := &state.Run{Worktree: wt, PRD: prdPath}
	title, body, err := pullRequestText(target, prdName, r)
	if err != nil || title != "Token für Apps" || body != "Scope: tokens\n\nPRD: `"+prdPath+"`\n" {
		t.Errorf("pullRequestText = %q, %q, %v", title, body, err)
	}
}
