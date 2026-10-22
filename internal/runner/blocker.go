package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"lazydev/internal/git"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// recordBlocker sets the status line of the PRD in the worktree to blockiert, writes the open
// questions as "## Blocker" and commits the PRD alone without Claude. Other uncommitted changes of
// the step stay uncommitted.
func recordBlocker(prdName string, r *state.Run, b *blockedError, log *logger) error {
	path := filepath.Join(r.Worktree, r.PRD)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("blocker not recorded in the PRD: %w", err)
	}
	value := fmt.Sprintf("%s (%s)", project.ValueBlocked, time.Now().Format("2006-01-02"))
	updated, ok := project.SetStatus(string(data), value)
	if !ok {
		return fmt.Errorf("blocker not recorded in the PRD: no status line in %s", r.PRD)
	}
	updated = project.SetBlocker(updated, b.step, b.points)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return err
	}
	msg := "docs(prd): record blocker of " + prdName
	if _, err := git.Run(r.Worktree, "commit", "-m", msg, "--", r.PRD); err != nil {
		os.WriteFile(path, data, 0o644)
		return err
	}
	log.printf("Commit: %s", msg)
	return nil
}

// checkBlocked refuses a run whose PRD in the worktree is still blockiert. The dialog of f
// resolves it first.
func checkBlocked(t Target, prdName string) error {
	r, err := state.Load(t.ID, prdName)
	if err == nil && project.BlockedIn(r.Worktree, r.PRD) {
		return fmt.Errorf("%s is still blocked", prdName)
	}
	return nil
}

// commitResolvedPRD commits the PRD without Claude when it is the only uncommitted file in the
// worktree, e.g. after the dialog of f resolved a blocked merge.
func commitResolvedPRD(prdName string, r *state.Run, log *logger) error {
	out, err := git.Run(r.Worktree, "status", "--porcelain")
	if err != nil {
		return err
	}
	lines := git.Lines(out)
	if len(lines) != 1 || len(lines[0]) < 4 || lines[0][3:] != r.PRD {
		return nil
	}
	msg := "docs(prd): resolve blocker of " + prdName
	if _, err := git.Run(r.Worktree, "commit", "-m", msg, "--", r.PRD); err != nil {
		return err
	}
	log.printf("Commit: %s", msg)
	return nil
}
