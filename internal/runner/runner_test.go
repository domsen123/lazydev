package runner

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"lazydev/internal/claude"
	"lazydev/internal/fsutil"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestRunMerges(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	wt := WorktreePath("app", prdName)
	if r.Phase != project.PhaseMerged || r.PID != 0 || r.Error != nil || r.Note != "" {
		t.Errorf("state: %+v", r)
	}
	if r.Worktree != wt || r.Branch != "feat/new-token" || r.Base != "main" || r.Slug != "new_token" || r.PRD != prdPath || r.Port < PortBase {
		t.Errorf("run fields: %+v", r)
	}
	if len(r.Steps) != 4 {
		t.Fatalf("steps: %+v", r.Steps)
	}
	for i, name := range state.Steps {
		if s := r.Steps[i]; s.Name != name || s.ExitCode == nil || *s.ExitCode != 0 || !s.OK || s.EndedAt == "" {
			t.Errorf("step %d: %+v", i, s)
		}
	}
	if s := r.Steps[1]; s.SessionID != "sess-1" || s.Model != "claude-opus-5-5" || s.CostUSD == nil || *s.CostUSD != 0.5 {
		t.Errorf("implement: %+v", s)
	}
	if s := r.Steps[3]; s.CostUSD != nil {
		t.Errorf("merge without Claude has cost: %+v", s)
	}

	main := target.Dir
	if got := runGit(t, main, "log", "--format=%s"); got != "docs(prd): record cost of new-token\nfeat: review\ndocs(prd): add new-token\ninit" {
		t.Errorf("main: %q", got)
	}
	if runGit(t, main, "rev-list", "--merges", "--count", "HEAD") != "0" {
		t.Error("merge commit on main")
	}
	if got := read(t, filepath.Join(main, prdPath)); !strings.Contains(got, "> Status: umgesetzt (2026-10-20) · Kosten: 0.80 USD · Modell: claude-opus-5-5 · Scope: tokens\n") {
		t.Errorf("PRD on main:\n%s", got)
	}
	if got := runGit(t, main, "status", "--porcelain"); got != "" {
		t.Errorf("main not clean: %q", got)
	}
	if fsutil.Exists(wt) {
		t.Error("worktree still exists")
	}
	if got := runGit(t, main, "branch", "--list", "feat/new-token"); got != "" {
		t.Errorf("branch still exists: %q", got)
	}
	if got := runGit(t, main, "worktree", "list", "--porcelain"); strings.Contains(got, wt) {
		t.Errorf("worktree still registered:\n%s", got)
	}

	realWT := wt
	hooks := read(t, filepath.Join(main, "hooks.log"))
	want := "setup " + realWT + " " + strconv.Itoa(r.Port) + " new_token feat/new-token\ncheck " + realWT + "\nteardown " + realWT + "\n"
	if hooks != want {
		t.Errorf("hooks.log:\n%s\nwant:\n%s", hooks, want)
	}

	setupLog := read(t, state.LogPath("app", prdName, state.StepSetup))
	for _, s := range []string{`"type":"lazydev"`, "Step setup", "PRD prd/auth/new-token.md moved into the worktree", "Commit: docs(prd): add new-token", "setup output", "Setup done: " + wt + " on port " + strconv.Itoa(r.Port)} {
		if !strings.Contains(setupLog, s) {
			t.Errorf("setup log lacks %q:\n%s", s, setupLog)
		}
	}
	implLog := read(t, state.LogPath("app", prdName, state.StepImplement))
	for _, s := range []string{`{"ts":"`, `"type":"result"`, `"type":"stderr"`, "eine Warnung", "# lazydev implement: prd/auth/new-token.md", "implement done"} {
		if !strings.Contains(implLog, s) {
			t.Errorf("implement log lacks %q:\n%s", s, implLog)
		}
	}
	mergeLog := read(t, state.LogPath("app", prdName, state.StepMerge))
	for _, s := range []string{"$ git rebase main", "check output", "Cost recorded in the PRD: 0.80 USD claude-opus-5-5", "Commit: docs(prd): record cost of new-token", "$ git merge --ff-only feat/new-token", "Removing worktree", "Merged"} {
		if !strings.Contains(mergeLog, s) {
			t.Errorf("merge log lacks %q:\n%s", s, mergeLog)
		}
	}
	if strings.Contains(mergeLog, "lazydev merge") {
		t.Errorf("Claude step merge ran without conflict:\n%s", mergeLog)
	}

	args := fakeArgs(t)
	for _, s := range []string{
		"-p\n# lazydev implement: prd/auth/new-token.md\n", "# lazydev review: feat/new-token → main\n",
		"## Regeln von lazydev", "## Pflichtlektüre", "- `CLAUDE.md`", "--output-format\nstream-json\n--verbose\n",
		"Bash(git status:*),", "Bash(make test:*)", "--agents\n" + filepath.Join(state.Dir(), "agents.json") + "\n",
		"GIT_EDITOR=true", "PWD=" + realWT + "\n",
	} {
		if !strings.Contains(args, s) {
			t.Errorf("claude args lack %q:\n%s", s, args)
		}
	}
	if strings.Contains(args, "# lazydev merge:") {
		t.Error("Claude step merge ran")
	}

	if err := Run(context.Background(), target, prdName); err == nil || !strings.Contains(err.Error(), "nothing left to do") {
		t.Errorf("Run after the merge: %v", err)
	}
}

func TestRunWithoutConfig(t *testing.T) {
	target := newProject(t, "", false)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged {
		t.Errorf("phase %s", r.Phase)
	}
	if args := fakeArgs(t); strings.Contains(args, "## Pflichtlektüre") {
		t.Errorf("knowledge block without knowledge:\n%s", args)
	}
	if fsutil.Exists(filepath.Join(target.Dir, "hooks.log")) {
		t.Error("hooks ran without config")
	}
}

func TestRunNotTrusted(t *testing.T) {
	target := newProject(t, hooksConfig, false)
	err := Run(context.Background(), target, prdName)
	if err == nil || !strings.Contains(err.Error(), "not trusted") {
		t.Fatalf("Run: %v", err)
	}
	if _, err := state.Load("app", prdName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("state without trust: %v", err)
	}
	if fsutil.Exists(WorktreePath("app", prdName)) {
		t.Error("worktree without trust")
	}
	if fakeArgs(t) != "" {
		t.Error("claude ran without trust")
	}

	trustProject(t, target)
	if err := target.CheckTrust(); err != nil {
		t.Errorf("CheckTrust after trust: %v", err)
	}
	// A changed hook needs trust again, other fields do not.
	write(t, target.Dir, ".lazydev/config.yaml", hooksConfig+"baseBranch: main\n")
	if err := target.CheckTrust(); err != nil {
		t.Errorf("CheckTrust after a change outside hooks: %v", err)
	}
	write(t, target.Dir, ".lazydev/config.yaml", strings.Replace(hooksConfig, "check.sh", "other.sh", 1))
	if err := target.CheckTrust(); err == nil {
		t.Error("CheckTrust accepts a changed hook")
	}
}

func TestRunBrokenConfig(t *testing.T) {
	target := newProject(t, "hooks: [\n", false)
	if err := Run(context.Background(), target, prdName); err == nil || !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("Run: %v", err)
	}
}

func TestRunWithoutClaude(t *testing.T) {
	target := newProject(t, "", false)
	claude.Command = []string{"claude-missing-in-test"}
	if err := Run(context.Background(), target, prdName); err == nil || err.Error() != "claude-missing-in-test not found, install Claude Code" {
		t.Errorf("Run: %v", err)
	}
	if _, err := state.Load("app", prdName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("state without claude: %v", err)
	}
}

func TestRunUnknownOrDonePRD(t *testing.T) {
	target := newProject(t, "", false)
	if err := Run(context.Background(), target, "missing"); err == nil || !strings.Contains(err.Error(), `no PRD "missing"`) {
		t.Errorf("unknown PRD: %v", err)
	}
	write(t, target.Dir, "prd/auth/done.md", "# PRD: Done\n\n> Status: umgesetzt (2026-10-01)\n")
	if err := Run(context.Background(), target, "done"); err == nil || !strings.Contains(err.Error(), "merged already") {
		t.Errorf("merged PRD: %v", err)
	}
	write(t, target.Dir, "prd/auth/gone.md", "# PRD: Gone\n\n> Status: verworfen (2026-10-01)\n")
	if err := Run(context.Background(), target, "gone"); err == nil || !strings.Contains(err.Error(), "discarded") {
		t.Errorf("discarded PRD: %v", err)
	}
	for _, name := range []string{"missing", "done", "gone"} {
		if _, err := state.Load("app", name); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("state for %s: %v", name, err)
		}
	}
}

func TestRunTrackedPRD(t *testing.T) {
	target := newProject(t, "", false)
	runGit(t, target.Dir, "add", prdPath)
	runGit(t, target.Dir, "commit", "-q", "-m", "docs(prd): add new-token by hand")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, target.Dir, "log", "--format=%s"); got != "docs(prd): record cost of new-token\nfeat: review\ndocs(prd): add new-token by hand\ninit" {
		t.Errorf("main: %q", got)
	}
}

func TestRunImplementAbortedContinues(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	t.Setenv("FAKE_CLAUDE_MODE", "abort")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "implement aborted: Feld fehlt im PRD" {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseError || r.Error == nil || *r.Error != err.Error() || r.PID != 0 {
		t.Errorf("state: %+v", r)
	}
	if r.NextStep() != state.StepImplement || len(r.Steps) != 2 || r.Steps[1].OK || *r.Steps[1].ExitCode != 0 {
		t.Errorf("steps: %+v", r.Steps)
	}
	if !fsutil.Exists(WorktreePath("app", prdName)) {
		t.Error("worktree removed after the error")
	}
	if log := read(t, state.LogPath("app", prdName, state.StepImplement)); !strings.Contains(log, "Error: implement aborted: Feld fehlt im PRD") {
		t.Errorf("implement log:\n%s", log)
	}

	t.Setenv("FAKE_CLAUDE_MODE", "ok")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r = loadRun(t)
	if r.Phase != project.PhaseMerged || r.Error != nil || len(r.Steps) != 5 {
		t.Errorf("state after the second run: %+v", r)
	}
	// The setup ran once, the aborted implement costs too.
	if got := strings.Count(read(t, filepath.Join(target.Dir, "hooks.log")), "setup "); got != 1 {
		t.Errorf("setup hook ran %d times", got)
	}
	if got := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(got, "Kosten: 1.00 USD · Modell: claude-opus-5-5") {
		t.Errorf("PRD on main:\n%s", got)
	}
}

func TestRunStepFailures(t *testing.T) {
	cases := []struct {
		mode, step, err string
	}{
		{"exit", state.StepImplement, "claude ended with exit code 3: kaputt"},
		{"noresult", state.StepImplement, "claude ended without result event"},
		{"error", state.StepImplement, "claude reports an error: Rate limit"},
		{"status", state.StepImplement, `status line of prd/auth/new-token.md is "geplant" instead of „umgesetzt (date)“`},
		{"dirty", state.StepReview, "worktree after the review not clean:  M prd/auth/new-token.md, ?? review.txt"},
		{"lazydev", state.StepImplement, ".lazydev/ changed in the worktree: .lazydev/neu.md"},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			target := newProject(t, "", false)
			t.Setenv("FAKE_CLAUDE_MODE", c.mode)
			err := Run(context.Background(), target, prdName)
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Fatalf("Run: %v, want %q", err, c.err)
			}
			r := loadRun(t)
			last := r.LastStep()
			if r.Phase != project.PhaseError || r.PID != 0 || last.Name != c.step || last.OK || r.NextStep() != c.step {
				t.Errorf("state: %+v", r)
			}
			wantExit := 0
			if c.mode == "exit" {
				wantExit = 3
			}
			if *last.ExitCode != wantExit {
				t.Errorf("exit code %d, want %d", *last.ExitCode, wantExit)
			}
		})
	}
}

func TestRunSetupHookFails(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	write(t, target.Dir, "setup.sh", "#!/bin/sh\necho broken >&2\nexit 2\n")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "setup hook ended with exit code 2" {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseError || r.NextStep() != state.StepSetup || *r.LastStep().ExitCode != 1 {
		t.Errorf("state: %+v", r)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepSetup)); !strings.Contains(log, "broken") {
		t.Errorf("hook output missing in the log:\n%s", log)
	}
	// The setup is repeatable: worktree and moved PRD stay, the second run continues.
	write(t, target.Dir, "setup.sh", setupScript)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepSetup)); !strings.Contains(log, "exists already") {
		t.Errorf("second setup:\n%s", log)
	}
	if got := runGit(t, target.Dir, "log", "--format=%s", "-3"); got != "docs(prd): record cost of new-token\nfeat: review\ndocs(prd): add new-token" {
		t.Errorf("main: %q", got)
	}
}

func TestRunBranchExists(t *testing.T) {
	target := newProject(t, "", false)
	runGit(t, target.Dir, "branch", "feat/new-token")
	if err := Run(context.Background(), target, prdName); err == nil || err.Error() != "branch feat/new-token exists already" {
		t.Errorf("Run: %v", err)
	}
}

func TestRunCanceled(t *testing.T) {
	target := newProject(t, "", false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The setup does not look at ctx, but runStep marks it aborted afterwards.
	err := Run(ctx, target, prdName)
	if err == nil || err.Error() != "setup aborted" {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseAborted || r.PID != 0 || r.NextStep() != state.StepSetup || r.Error != nil {
		t.Errorf("state: %+v", r)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepSetup)); !strings.Contains(log, "Aborted") {
		t.Errorf("setup log:\n%s", log)
	}
	// The setup repeats on the existing worktree.
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged || len(r.Steps) != 5 {
		t.Errorf("state after the second run: %+v", r)
	}
}

func TestRunRefusesWhileAnotherRunnerRuns(t *testing.T) {
	target := newProject(t, "", false)
	pid := startFakeRunner(t)
	if err := state.Save("app", prdName, &state.Run{PID: pid, Phase: project.PhaseImplement, Steps: []state.Step{{Name: state.StepSetup}}}); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), target, prdName)
	if err == nil || !strings.Contains(err.Error(), "runs already (PID "+strconv.Itoa(pid)) {
		t.Errorf("Run: %v", err)
	}
	if err := Discard(target, prdName); err == nil || !strings.Contains(err.Error(), "still runs") {
		t.Errorf("Discard of a running PRD: %v", err)
	}
}

func TestStartWaitsAtMaxParallel(t *testing.T) {
	target := newProject(t, "", false)
	for i := range MaxParallel {
		name := "other-" + strconv.Itoa(i)
		if err := state.Save("app", name, &state.Run{PID: startFakeRunner(t), Phase: project.PhaseImplement, Port: PortBase + i}); err != nil {
			t.Fatal(err)
		}
	}
	started, err := Start(target, prdName)
	if err != nil || started {
		t.Fatalf("Start: %v, started %v", err, started)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseWaiting || r.PID != 0 || len(r.Steps) != 0 || r.Port < PortBase+MaxParallel {
		t.Errorf("state: %+v", r)
	}
	// A waiting PRD stays in the main folder until its runner starts.
	if !fsutil.Exists(filepath.Join(target.Dir, prdPath)) {
		t.Error("PRD moved before the start")
	}
	if err := Stop("app", prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseAborted {
		t.Errorf("phase after Stop: %s", r.Phase)
	}
}

func TestStop(t *testing.T) {
	isolate(t)
	if err := Stop("app", prdName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stop without state: %v", err)
	}
	if err := state.Save("app", prdName, &state.Run{Phase: project.PhaseWaiting}); err != nil {
		t.Fatal(err)
	}
	if err := Stop("app", prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseAborted {
		t.Errorf("phase: %s", r.Phase)
	}
	if err := Stop("app", prdName); err == nil || !strings.Contains(err.Error(), "no runner for new-token, phase aborted") {
		t.Errorf("Stop of an aborted run: %v", err)
	}
	// An active phase without a living runner shows as aborted.
	if err := state.Save("app", prdName, &state.Run{Phase: project.PhaseReview, PID: 999999999}); err != nil {
		t.Fatal(err)
	}
	if err := Stop("app", prdName); err == nil || !strings.Contains(err.Error(), "phase aborted") {
		t.Errorf("Stop of a dead runner: %v", err)
	}
}

func TestStopRunningRunner(t *testing.T) {
	isolate(t)
	pid := startFakeRunner(t)
	if err := state.Save("app", prdName, &state.Run{PID: pid, Phase: project.PhaseImplement}); err != nil {
		t.Fatal(err)
	}
	if err := Stop("app", prdName); err != nil {
		t.Fatal(err)
	}
	// An ended child the test has not waited for is a zombie with an empty command line.
	waitFor(t, "the runner to end", func() bool { return !state.IsRunner(pid) })
}
