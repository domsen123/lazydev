package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/notify"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
	"lazydev/internal/state"
)

// With auto-unblock on for a project ("autoUnblock" in settings.json), a blocked implement or merge
// does not stop the run right away. The step resolve answers the questions of "## Blocker" in the
// worktree without the user, commits the PRD alone and the blocked step runs again. If Claude may
// not decide a question, or the check of the step fails, the run ends in phase blocked as without
// auto-unblock.

// MaxResolves limits the resolve steps of one run, so a PRD that blocks again and again stops.
const MaxResolves = 2

// resultResolved is the value under "## Ergebnis" of a resolve that answered every question.
const resultResolved = "geklärt"

// resolveTools let resolve read the worktree and edit the PRD. Bash only reads git.
var resolveTools = []string{
	"Read", "Edit", "Write", "Glob", "Grep",
	"Bash(git status:*)", "Bash(git diff:*)", "Bash(git log:*)", "Bash(git show:*)",
}

// canResolve reports whether a blocked step of r gets the step resolve.
func canResolve(t Target, r *state.Run) bool {
	return config.LoadSettings().AutoUnblockEnabled(t.Dir) && r.Resolves() < MaxResolves
}

// resolve runs the step resolve for the blocked step b. On success the run goes on with the
// blocked step, otherwise it ends in phase blocked with the notification the block would have sent.
func resolve(ctx context.Context, t Target, prdName string, r *state.Run, b *blockedError) error {
	r.Phase, r.Error = project.PhaseResolve, nil
	r.StartStep(state.StepResolve)
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	log, err := openLog(state.LogPath(t.ID, prdName, state.StepResolve))
	if err != nil {
		return err
	}
	defer log.close()
	log.printf("Step resolve: %d open questions of %s, attempt %d of %d", len(b.points), b.step, r.Resolves(), MaxResolves)

	decisions, exitCode, err := resolveStep(ctx, t, prdName, r, log)
	if err != nil || ctx.Err() != nil {
		// The PRD goes back to the committed state with "## Blocker".
		if _, rerr := git.Run(r.Worktree, "checkout", "HEAD", "--", r.PRD); rerr != nil {
			log.printf("PRD not restored: %v", rerr)
		}
	}
	if ctx.Err() != nil {
		log.printf("Aborted")
		r.EndStep(exitCode, false)
		r.Phase, r.PID = project.PhaseAborted, 0
		saveErr := state.Save(t.ID, prdName, r)
		notifyRun(t, r, notify.Aborted, log)
		return errors.Join(errors.New("resolve aborted"), saveErr)
	}
	if err != nil {
		log.printf("Not resolved: %v", err)
		r.EndStep(exitCode, false)
		r.Block(b.Error())
		r.PID = 0
		saveErr := state.Save(t.ID, prdName, r)
		notifyRun(t, r, notify.Blocked, log, b.points...)
		return errors.Join(b, saveErr)
	}
	msg := "docs(prd): resolve blocker of " + prdName
	if _, err := git.Run(r.Worktree, "commit", "-m", msg, "--", r.PRD); err != nil {
		log.printf("Error: %v", err)
		r.EndStep(1, false)
		r.Fail(err)
		r.PID = 0
		return errors.Join(err, state.Save(t.ID, prdName, r))
	}
	log.printf("Commit: %s", msg)
	for _, d := range decisions {
		log.printf("Decision: %s", d)
	}
	log.printf("Resolved, %s runs again", b.step)
	r.EndStep(0, true)
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	notifyRun(t, r, notify.Resolved, log, decisions...)
	return nil
}

// resolveStep calls Claude with the resolve prompt in the worktree and checks the outcome: the
// report says geklärt, the PRD is no longer blocked and has no "## Blocker", and Claude changed
// nothing but the PRD. It returns the decisions of the report.
func resolveStep(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) ([]string, int, error) {
	before, err := git.Run(r.Worktree, "status", "--porcelain")
	if err != nil {
		return nil, 1, err
	}
	mode, err := config.LoadSettings().Mode()
	if err != nil {
		return nil, 1, err
	}
	prompt := prompts.Resolve(prompts.Vars{Arguments: r.PRD, Base: r.Base, Branch: r.Branch})
	s, exitCode, err := callClaude(ctx, t, prdName, r, r.Worktree, prompt, resolveTools, mode, log)
	if err != nil {
		return nil, exitCode, err
	}
	if err := checkSummary(s, exitCode); err != nil {
		return nil, exitCode, err
	}
	if err := checkReport(s.Result, state.StepResolve, resultResolved, "Offene Fragen"); err != nil {
		var b *blockedError
		if errors.As(err, &b) {
			return nil, 0, fmt.Errorf("Claude may not decide: %s", strings.Join(b.points, " | "))
		}
		return nil, 0, err
	}
	after, err := git.Run(r.Worktree, "status", "--porcelain")
	if err != nil {
		return nil, 0, err
	}
	old := git.Lines(before)
	for _, l := range git.Lines(after) {
		if len(l) > 3 && l[3:] != r.PRD && !slices.Contains(old, l) {
			return nil, 0, fmt.Errorf("resolve changed %s, it may only change the PRD", l[3:])
		}
	}
	if err := checkLazydevUntouched(r); err != nil {
		return nil, 0, err
	}
	d, err := project.Read(r.Worktree, filepath.Join(r.Worktree, r.PRD))
	if err != nil {
		return nil, 0, err
	}
	if d.Status.Kind == project.StatusBlocked {
		return nil, 0, fmt.Errorf("status line of %s still says %s", r.PRD, project.ValueBlocked)
	}
	if content, err := os.ReadFile(filepath.Join(r.Worktree, r.PRD)); err != nil {
		return nil, 0, err
	} else if _, _, ok := project.Blocker(string(content)); ok {
		return nil, 0, fmt.Errorf("%s still has ## Blocker", r.PRD)
	}
	section, _ := claude.Section(s.Result, "Entscheidungen")
	return reportPoints(section), 0, nil
}
