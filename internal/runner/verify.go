package runner

import (
	"context"
	"os"

	"lazydev/internal/devserver"
	"lazydev/internal/fsutil"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// verify runs the step verify: Claude checks the feature in the running app and puts evidence into
// the evidence folder of the run. It is opt-in, without .lazydev/verify.md it ends without Claude.
// With hooks.dev the runner starts the dev server for the step and stops it afterwards, on error
// and abort too.
func verify(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) (int, error) {
	if !fsutil.Exists(repo.PromptPath(t.Dir, repo.StepVerify)) {
		log.printf("No %s/%s.md, verify skipped", repo.Dir, repo.StepVerify)
		return 0, nil
	}
	if err := os.MkdirAll(state.EvidenceDir(t.ID, prdName), 0o755); err != nil {
		return 1, err
	}
	cfg, err := t.Repo()
	if err != nil {
		return 1, err
	}
	if cfg.Hooks.Dev != "" {
		// A dev server started with D may run older code, verify starts it again.
		if err := devserver.Stop(t.ID, prdName); err != nil {
			return 1, err
		}
		pid, err := StartDev(t, prdName)
		if err != nil {
			return 1, err
		}
		log.printf("Dev server started for verify (PID %d, port %d)", pid, r.Port)
		defer func() {
			if err := devserver.Stop(t.ID, prdName); err != nil {
				log.printf("Dev server not stopped: %v", err)
				return
			}
			log.printf("Dev server stopped")
		}()
	}
	return claudeStep(ctx, t, prdName, r, state.StepVerify, log)
}
