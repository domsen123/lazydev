package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// Values under "## Ergebnis" in the reports of the Claude steps, set by the contracts.
const (
	resultImplemented = "umgesetzt"
	resultAborted     = "abgebrochen"
	resultMergeReady  = "bereit für fast-forward"
)

// claudeStep runs implement or review in the worktree and checks the outcome: after implement the
// report and the status line of the PRD, after review a clean worktree.
func claudeStep(ctx context.Context, t Target, prdName string, r *state.Run, step string, log *logger) (int, error) {
	args := ""
	if step == state.StepImplement {
		args = r.PRD
	}
	s, exitCode, err := runClaude(ctx, t, prdName, r, step, args, log)
	if err != nil {
		return exitCode, err
	}
	if err := checkSummary(s, exitCode); err != nil {
		return exitCode, err
	}
	if step == state.StepImplement {
		if err := checkReport(s.Result, step, resultImplemented, "Offene Fragen"); err != nil {
			return 0, err
		}
		d, err := project.Read(r.Worktree, filepath.Join(r.Worktree, r.PRD))
		if err != nil {
			return 0, err
		}
		if d.Status.Kind != project.StatusImplemented {
			return 0, fmt.Errorf("status line of %s is %q instead of „%s (date)“", r.PRD, d.Status.Value, project.ValueImplemented)
		}
	} else if err := requireClean(r.Worktree, "worktree after the review"); err != nil {
		return 0, err
	}
	return 0, checkLazydevUntouched(r)
}

// runClaude builds the prompt of the step and starts claude in the worktree. Session, model and
// cost go into the last step as they come. Several calls in one step add up their cost.
func runClaude(ctx context.Context, t Target, prdName string, r *state.Run, step, args string, log *logger) (claude.Summary, int, error) {
	cfg, err := t.Repo()
	if err != nil {
		return claude.Summary{}, 1, err
	}
	// Every step reads settings.json anew, so a change applies from the next step on.
	settings := config.LoadSettings()
	mode, err := settings.Mode()
	if err != nil {
		return claude.Summary{}, 1, err
	}
	tools := allowedTools(claude.DefaultAllowedTools, settings.AllowedTools, cfg.AllowedTools)
	prompt, err := prompts.Build(t.Dir, r.Worktree, cfg, step, prompts.Vars{Arguments: args, Base: r.Base, Branch: r.Branch})
	if err != nil {
		return claude.Summary{}, 1, err
	}
	baseCost := 0.0
	if c := r.LastStep().CostUSD; c != nil {
		baseCost = *c
	}
	o := claude.Options{
		Dir:            r.Worktree,
		Prompt:         prompt,
		AllowedTools:   tools,
		PermissionMode: mode,
		Log:            log,
		OnChange: func(s claude.Summary) {
			last := r.LastStep()
			last.SessionID = s.SessionID
			if s.Model != "" {
				last.Model = s.Model
			}
			if s.HasResult {
				cost := baseCost + s.CostUSD
				last.CostUSD = &cost
			}
			if err := state.Save(t.ID, prdName, r); err != nil {
				log.printf("State not saved: %v", err)
			}
		},
	}
	log.printf("Permissions: %s, %d allowed tools", mode, len(tools))
	log.printf("$ %s %s", claude.Command[0], claude.FirstLine(prompt))
	s, exitCode, err := claude.Run(ctx, o)
	if err != nil {
		return s, exitCode, fmt.Errorf("%s: %w", claude.Command[0], err)
	}
	return s, exitCode, nil
}

// allowedTools joins the lists in their order without duplicates, the first entry stays.
func allowedTools(lists ...[]string) []string {
	var tools []string
	for _, list := range lists {
		for _, tool := range list {
			if !slices.Contains(tools, tool) {
				tools = append(tools, tool)
			}
		}
	}
	return tools
}

// checkSummary evaluates exit code and the last result event.
func checkSummary(s claude.Summary, exitCode int) error {
	switch {
	case exitCode != 0 && s.Result != "":
		return fmt.Errorf("claude ended with exit code %d: %s", exitCode, claude.FirstLine(s.Result))
	case exitCode != 0:
		return fmt.Errorf("claude ended with exit code %d", exitCode)
	case !s.HasResult:
		return errors.New("claude ended without result event")
	case s.IsError:
		return fmt.Errorf("claude reports an error: %s", claude.FirstLine(s.Result))
	}
	return nil
}

// blockedError is a step that Claude aborted on purpose with open questions. It needs a decision of
// the user, not another attempt: the run ends in phase blocked.
type blockedError struct {
	step   string
	points []string
}

func (e *blockedError) Error() string {
	return fmt.Sprintf("%s blocked: %s", e.step, e.points[0])
}

// checkReport checks "## Ergebnis" in the report of a step. If it is abgebrochen with points in the
// section questions, the error is a *blockedError.
func checkReport(report, step, want, questions string) error {
	res, ok := claude.Section(report, "Ergebnis")
	if !ok {
		return fmt.Errorf("report of %s without „## Ergebnis“", step)
	}
	switch v := strings.ToLower(claude.FirstLine(res)); v {
	case want:
		return nil
	case resultAborted:
		q, _ := claude.Section(report, questions)
		if points := reportPoints(q); len(points) > 0 {
			return &blockedError{step: step, points: points}
		}
		return fmt.Errorf("%s aborted", step)
	default:
		return fmt.Errorf("unknown result %q in the report of %s", v, step)
	}
}

// reportPoints are the lines of a report section without "- ", empty lines and „keine“.
func reportPoints(section string) []string {
	var points []string
	for line := range strings.SplitSeq(section, "\n") {
		p := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if p != "" && p != "keine" {
			points = append(points, p)
		}
	}
	return points
}

// checkLazydevUntouched makes sure Claude changed nothing under .lazydev/ in the worktree, neither
// in commits since the base branch nor uncommitted.
func checkLazydevUntouched(r *state.Run) error {
	committed, err := git.Run(r.Worktree, "diff", "--name-only", r.Base+"...HEAD", "--", repo.Dir)
	if err != nil {
		return err
	}
	status, err := git.Run(r.Worktree, "status", "--porcelain", "--untracked-files=all", "--", repo.Dir)
	if err != nil {
		return err
	}
	changed := git.Lines(committed)
	for _, line := range git.Lines(status) {
		if len(line) > 3 && !slices.Contains(changed, line[3:]) {
			changed = append(changed, line[3:])
		}
	}
	if len(changed) > 0 {
		return fmt.Errorf("%s/ changed in the worktree: %s", repo.Dir, strings.Join(changed, ", "))
	}
	return nil
}
