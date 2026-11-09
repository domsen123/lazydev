package runner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/fsutil"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
	"lazydev/internal/state"
)

// The step plan comes before the setup. It decides whether the PRD may start next to the open runs
// of its project or must wait until some of them are merged, and stores that in Run.After. Without
// other open runs there is nothing to decide and no Claude call. The setup of a PRD starts only
// when every PRD of After is merged or its run is gone.

// planTools are the only tools of the step plan: it reads and changes nothing.
var planTools = []string{"Read", "Glob", "Grep"}

// planMode lets claude -p deny every tool outside planTools instead of accepting edits.
const planMode = "default"

// planPhases are the phases of runs the step plan weighs a new PRD against: they work, wait or
// continue later.
var planPhases = []project.Phase{
	project.PhaseWaiting, project.PhasePlan, project.PhaseSetup, project.PhaseImplement,
	project.PhaseReview, project.PhaseMergeQueue, project.PhaseMerge, project.PhaseBlocked,
}

// plan runs the step plan for the PRD of r.
func plan(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) (int, error) {
	runs := planCandidates(t.ID, prdName)
	if len(runs) == 0 {
		r.After, r.AfterReason = nil, ""
		log.printf("No other open runs in %s, %s starts right away", t.ID, prdName)
		return 0, nil
	}
	names := make([]string, len(runs))
	for i, c := range runs {
		names[i] = c.Name
	}
	log.printf("Open runs: %s", strings.Join(names, ", "))
	prompt := prompts.Plan(prompts.Vars{Arguments: r.PRD, Base: r.Base, Branch: r.Branch}, runs)
	s, exitCode, err := callClaude(ctx, t, prdName, r, t.Dir, prompt, planTools, planMode, log)
	if err != nil {
		return exitCode, err
	}
	if err := checkSummary(s, exitCode); err != nil {
		return exitCode, err
	}
	after, reason, err := parsePlan(s.Result, names)
	if err != nil {
		return 0, err
	}
	if dropped := dropCycles(t.ID, prdName, &after); len(dropped) > 0 {
		log.printf("Not waiting for %s, they wait for %s themselves", strings.Join(dropped, ", "), prdName)
	}
	r.After, r.AfterReason = after, reason
	if len(after) == 0 {
		log.printf("Parallel: %s", reason)
	} else {
		log.printf("After %s: %s", strings.Join(after, ", "), reason)
	}
	return 0, nil
}

// planCandidates lists the open runs of project id other than prdName with the path of their PRD:
// in the worktree once the run moved it there, otherwise in the main folder.
func planCandidates(id, prdName string) []prompts.PlanRun {
	var runs []prompts.PlanRun
	for _, e := range state.All() {
		if e.ID != id || e.PRDName == prdName || !slices.Contains(planPhases, e.Run.Effective()) {
			continue
		}
		file := filepath.Join(e.Run.Worktree, e.Run.PRD)
		if !fsutil.Exists(file) {
			file = filepath.Join(e.Run.ProjectDir, e.Run.PRD)
		}
		runs = append(runs, prompts.PlanRun{Name: e.PRDName, Phase: string(e.Run.Effective()), File: file})
	}
	return runs
}

// parsePlan reads the section "## Reihenfolge" at the end of the answer: "Nach: <prd>, …" or
// "Nach: keine" and "Grund: <sentence>". Names may come with backticks, area or ".md"; names not
// in known are an error, so a typo does not start a PRD too early.
func parsePlan(result string, known []string) (after []string, reason string, err error) {
	i := strings.LastIndex(result, "## Reihenfolge")
	if i < 0 {
		return nil, "", errors.New("the answer of plan lacks ## Reihenfolge")
	}
	var afterLine string
	hasAfter := false
	for l := range strings.SplitSeq(result[i:], "\n") {
		l = strings.TrimSpace(strings.Trim(strings.TrimSpace(l), "`"))
		if v, ok := strings.CutPrefix(l, "Nach:"); ok {
			afterLine, hasAfter = strings.TrimSpace(v), true
		} else if v, ok := strings.CutPrefix(l, "Grund:"); ok {
			reason = strings.TrimSpace(v)
		}
	}
	if !hasAfter {
		return nil, "", errors.New("the answer of plan lacks the line Nach:")
	}
	if strings.EqualFold(strings.Trim(afterLine, "`. "), "keine") || afterLine == "" {
		return nil, reason, nil
	}
	for part := range strings.SplitSeq(afterLine, ",") {
		name := strings.Trim(strings.TrimSpace(part), "`. ")
		name = strings.TrimSuffix(filepath.Base(name), ".md")
		if !slices.Contains(known, name) {
			return nil, "", fmt.Errorf("plan names %q, which is no open run", strings.TrimSpace(part))
		}
		if !slices.Contains(after, name) {
			after = append(after, name)
		}
	}
	return after, reason, nil
}

// dropCycles removes the PRDs from after that wait for prdName themselves, directly or through
// others, and returns them.
func dropCycles(id, prdName string, after *[]string) (dropped []string) {
	kept := (*after)[:0]
	for _, dep := range *after {
		if waitsFor(id, dep, prdName, map[string]bool{}) {
			dropped = append(dropped, dep)
		} else {
			kept = append(kept, dep)
		}
	}
	*after = kept
	return dropped
}

// waitsFor reports whether the run of name waits for target through the After lists of project id.
func waitsFor(id, name, target string, seen map[string]bool) bool {
	if seen[name] {
		return false
	}
	seen[name] = true
	r, err := state.Load(id, name)
	if err != nil {
		return false
	}
	for _, dep := range r.After {
		if dep == target || waitsFor(id, dep, target, seen) {
			return true
		}
	}
	return false
}

// PendingAfter lists the PRDs of r.After that are not merged yet. A PRD without run state counts
// as done: its run was discarded.
func PendingAfter(id string, r *state.Run) []string {
	var pending []string
	for _, dep := range r.After {
		d, err := state.Load(id, dep)
		if errors.Is(err, fs.ErrNotExist) || (err == nil && d.Phase == project.PhaseMerged) {
			continue
		}
		pending = append(pending, dep)
	}
	return pending
}

// waitsForAfter is true when the next step of r is the setup and a PRD of After is not merged.
func waitsForAfter(id string, r *state.Run) bool {
	return r.NextStep() == state.StepSetup && len(PendingAfter(id, r)) > 0
}

// SetAfter is "lazydev after": it sets After of a run that has not started its setup yet by hand,
// marks the step plan as done and starts waiting PRDs whose turn it is now.
func SetAfter(t Target, prdName string, after []string) error {
	unlock, err := state.Lock(startLock)
	if err != nil {
		return err
	}
	r, err := state.Load(t.ID, prdName)
	if errors.Is(err, fs.ErrNotExist) {
		unlock()
		return fmt.Errorf("%s has no run, start it first", prdName)
	} else if err != nil {
		unlock()
		return err
	}
	if next := r.NextStep(); next != state.StepPlan && next != state.StepSetup {
		unlock()
		return fmt.Errorf("%s has started already, phase %s", prdName, r.Effective())
	}
	if r.Running() && r.Phase == project.PhasePlan {
		unlock()
		return fmt.Errorf("%s plans right now, set it once plan is done", prdName)
	}
	var deps []string
	for _, dep := range after {
		dep = strings.TrimSuffix(filepath.Base(dep), ".md")
		if dep == prdName {
			unlock()
			return fmt.Errorf("%s cannot wait for itself", prdName)
		}
		if _, err := state.Load(t.ID, dep); err != nil {
			unlock()
			return fmt.Errorf("%s has no run in %s", dep, t.ID)
		}
		if waitsFor(t.ID, dep, prdName, map[string]bool{}) {
			unlock()
			return fmt.Errorf("%s waits for %s already, that would be a cycle", dep, prdName)
		}
		if !slices.Contains(deps, dep) {
			deps = append(deps, dep)
		}
	}
	r.After, r.AfterReason = deps, "set by hand"
	if r.NextStep() == state.StepPlan {
		r.StartStep(state.StepPlan)
		r.EndStep(0, true)
	}
	err = state.Save(t.ID, prdName, r)
	unlock()
	if err != nil {
		return err
	}
	return startWaiting()
}

// WaitReason says why a PRD that Start left waiting waits: for the PRDs of After, or for a free
// slot.
func WaitReason(id, prdName string) string {
	if r, err := state.Load(id, prdName); err == nil && r.NextStep() == state.StepSetup {
		if pending := PendingAfter(id, r); len(pending) > 0 {
			return fmt.Sprintf("%s waits for %s: %s", prdName, strings.Join(pending, ", "), r.AfterReason)
		}
	}
	return fmt.Sprintf("%d runners work already. %s waits and starts once one ends.", MaxParallel, prdName)
}
