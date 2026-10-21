package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazydev/internal/devserver"
	"lazydev/internal/git"
	"lazydev/internal/notify"
	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// MergePollInterval is how often a PRD in the merge queue checks for the lock or a clean main
// folder. Tests shorten it.
var MergePollInterval = 5 * time.Second

// merge rebases the branch onto the base branch, checks it and fast-forwards the main folder. Per
// project only one runner merges, the others wait in the merge queue. Merges always run right
// after the review.
func merge(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) error {
	unlock, err := waitMergeLock(ctx, t.ID, log)
	if err != nil {
		return err
	}
	defer unlock()
	r.Phase, r.Note = project.PhaseMerge, ""
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	if pid := devserver.PID(t.ID, prdName); pid != 0 {
		log.printf("Stopping the dev server of the worktree (PID %d)", pid)
		if err := devserver.Stop(t.ID, prdName); err != nil {
			return err
		}
	}
	wt := r.Worktree
	if err := requireClean(wt, "worktree"); err != nil {
		return err
	}
	log.printf("$ git rebase %s", r.Base)
	if _, err := git.Run(wt, "rebase", r.Base); err != nil {
		log.printf("Conflict in the rebase: %v", err)
		if _, err := git.Run(wt, "rebase", "--abort"); err != nil {
			log.printf("git rebase --abort: %v", err)
		}
		if err := mergeClaude(ctx, t, prdName, r, "", log); err != nil {
			return err
		}
	}
	if err := runHook(t, r, log, "check"); err != nil {
		log.printf("Check red: %v", err)
		if err := mergeClaude(ctx, t, prdName, r, "Prüfung nach dem Rebase fehlgeschlagen: "+err.Error(), log); err != nil {
			return err
		}
		if err := runHook(t, r, log, "check"); err != nil {
			return fmt.Errorf("check still red after the Claude step merge: %w", err)
		}
	}
	if err := recordRunInfo(prdName, r, log); err != nil {
		return err
	}
	if err := waitMainClean(ctx, t, prdName, r, log); err != nil {
		return err
	}
	log.printf("$ git merge --ff-only %s", r.Branch)
	if _, err := git.Run(t.Dir, "merge", "--ff-only", r.Branch); err != nil {
		return err
	}
	log.printf("%s is at %s", r.Base, r.Branch)
	var errs []error
	if err := runHook(t, r, log, "teardown"); err != nil {
		errs = append(errs, err)
	}
	log.printf("Removing worktree %s and branch %s", r.Worktree, r.Branch)
	if err := removeWorktree(t.Dir, r.Worktree, r.Branch, false); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		log.printf("Cleanup incomplete: %v", err)
		r.Note = "cleanup incomplete: " + err.Error()
	}
	return nil
}

// mergeClaude lets Claude bring the branch onto the base branch. context explains why.
func mergeClaude(ctx context.Context, t Target, prdName string, r *state.Run, context string, log *logger) error {
	args := r.Branch
	if context != "" {
		args += " " + context
	}
	s, exitCode, err := runClaude(ctx, t, prdName, r, state.StepMerge, args, log)
	if err == nil {
		err = checkSummary(s, exitCode)
	}
	if err == nil {
		err = checkReport(s.Result, state.StepMerge, resultMergeReady, "Offene Punkte")
	}
	if err != nil {
		return err
	}
	// The report alone is not enough, the branch must really sit on the base branch.
	if _, err := git.Run(r.Worktree, "merge-base", "--is-ancestor", r.Base, "HEAD"); err != nil {
		return fmt.Errorf("after the Claude step merge, %s is not on %s", r.Branch, r.Base)
	}
	if err := requireClean(r.Worktree, "worktree after the Claude step merge"); err != nil {
		return err
	}
	return checkLazydevUntouched(r)
}

// recordRunInfo writes cost and model of the run into the status line of the PRD in the worktree
// and commits it without Claude. The check does not run again, the commit only changes the PRD.
// Without a change there is no commit.
func recordRunInfo(prdName string, r *state.Run, log *logger) error {
	path := filepath.Join(r.Worktree, r.PRD)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cost not recorded in the PRD: %w", err)
	}
	cost, model := r.Cost(), r.ImplementModel()
	updated, ok := project.SetRunInfo(string(data), cost, model)
	if !ok {
		return fmt.Errorf("cost not recorded in the PRD: no status line in %s", r.PRD)
	}
	log.printf("Cost recorded in the PRD: %.2f USD %s", cost, model)
	if updated == string(data) {
		return nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return err
	}
	msg := "docs(prd): record cost of " + prdName
	if _, err := git.Run(r.Worktree, "commit", "-m", msg, "--", r.PRD); err != nil {
		// With the old content back, the next attempt does not fail on a dirty worktree.
		os.WriteFile(path, data, 0o644)
		return err
	}
	log.printf("Commit: %s", msg)
	return nil
}

// waitMainClean waits until the main folder is on the base branch and clean. Untracked PRDs and
// files under .lazydev/ do not count.
func waitMainClean(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) error {
	waiting := false
	for {
		problem, err := mainDirProblem(t.Dir, r.Base)
		if err != nil {
			return err
		}
		if problem == "" {
			if waiting {
				r.Phase, r.Note = project.PhaseMerge, ""
				return state.Save(t.ID, prdName, r)
			}
			return nil
		}
		if !waiting {
			log.printf("Main folder blocks the merge: %s", problem)
			r.Phase, r.Note = project.PhaseMergeQueue, "main folder: "+problem
			if err := state.Save(t.ID, prdName, r); err != nil {
				return err
			}
			notifyRun(t, r, notify.MainFolder, log)
			waiting = true
		}
		if err := sleep(ctx, MergePollInterval); err != nil {
			return err
		}
	}
}

// mainDirProblem describes what blocks the fast-forward in the main folder, empty for nothing.
func mainDirProblem(dir, base string) (string, error) {
	branch, err := git.Run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if branch != base {
		return fmt.Sprintf("on %s instead of %s", branch, base), nil
	}
	out, err := git.Run(dir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	var dirty []string
	for _, line := range git.Lines(out) {
		if len(line) < 4 {
			continue
		}
		path := line[3:]
		if (strings.HasPrefix(line, "?? ") && strings.HasPrefix(path, project.Dir+"/")) || strings.HasPrefix(path, repo.Dir+"/") {
			continue
		}
		dirty = append(dirty, path)
	}
	if len(dirty) > 0 {
		return "changed: " + strings.Join(dirty, ", "), nil
	}
	return "", nil
}

// waitMergeLock waits for the merge slot of the project. ctx ends the waiting.
func waitMergeLock(ctx context.Context, id string, log *logger) (func(), error) {
	logged := false
	for {
		unlock, err := state.TryLock(state.MergeLock(id))
		if err != nil || unlock != nil {
			return unlock, err
		}
		if !logged {
			log.printf("Another PRD merges, waiting for the merge slot")
			logged = true
		}
		if err := sleep(ctx, MergePollInterval); err != nil {
			return nil, err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
