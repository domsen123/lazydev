package runner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"lazydev/internal/devserver"
	"lazydev/internal/fsutil"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// Discard throws away the run of a PRD: dev server, teardown hook, worktree, branch and state. A
// PRD that is missing in the main folder goes back there untracked with status geplant first. It
// works without state too, so a half created run can be cleaned up.
func Discard(t Target, prdName string) error {
	r, err := state.Load(t.ID, prdName)
	if errors.Is(err, fs.ErrNotExist) {
		r = &state.Run{ProjectDir: t.Dir, Branch: project.FeaturePrefix + prdName, Worktree: WorktreePath(t.ID, prdName), Slug: Slug(prdName)}
	} else if err != nil {
		return err
	}
	if r.Running() {
		return fmt.Errorf("%s still runs, stop it first", prdName)
	}
	log, _ := openLog("")
	var errs []error
	if err := devserver.Stop(t.ID, prdName); err != nil {
		errs = append(errs, err)
	}
	if fsutil.Exists(r.Worktree) {
		if err := runHook(t, r, log, "teardown"); err != nil {
			errs = append(errs, err)
		}
	}
	if err := restorePRD(t.Dir, r.Worktree, r.PRD); err != nil {
		// Without the PRD saved, the worktree stays, so nothing gets lost.
		return errors.Join(append(errs, err)...)
	}
	if err := removeWorktree(t.Dir, r.Worktree, r.Branch, true); err != nil {
		errs = append(errs, err)
	}
	if err := state.Remove(t.ID, prdName); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// restorePRD puts the PRD from the worktree back into the main folder if it is missing there, with
// status geplant and without "## Blocker", because the implementation is gone with the worktree.
func restorePRD(mainDir, wt, rel string) error {
	src, dst := filepath.Join(wt, rel), filepath.Join(mainDir, rel)
	if rel == "" || !fsutil.Exists(src) || fsutil.Exists(dst) {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	reset, _ := project.SetStatus(string(data), project.ValuePlanned)
	return os.WriteFile(dst, []byte(project.RemoveBlocker(reset)), 0o644)
}
