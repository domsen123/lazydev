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
	"lazydev/internal/fsutil"
	"lazydev/internal/git"
	"lazydev/internal/lang"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// word picks a word of a vocabulary. Reports are read in every language, so a step does not fail
// when Claude answers in the other one.
type word func(lang.Vocab) string

// The headings and values of the reports of the Claude steps, set by the contracts.
var (
	headingResult        word = func(v lang.Vocab) string { return v.Result }
	headingOpenQuestions word = func(v lang.Vocab) string { return v.OpenQuestions }
	headingOpenPoints    word = func(v lang.Vocab) string { return v.OpenPoints }
	headingEvidence      word = func(v lang.Vocab) string { return v.Evidence }
	headingDecisions     word = func(v lang.Vocab) string { return v.Decisions }
	resultImplemented    word = func(v lang.Vocab) string { return v.ResultImplemented }
	resultVerified       word = func(v lang.Vocab) string { return v.ResultVerified }
	resultAborted        word = func(v lang.Vocab) string { return v.ResultAborted }
	resultMergeReady     word = func(v lang.Vocab) string { return v.ResultMergeReady }
	resultResolved       word = func(v lang.Vocab) string { return v.ResultResolved }
)

// reportSection is the section of report under the heading w in any language, the first language
// that has it wins.
func reportSection(report string, w word) (string, bool) {
	for _, v := range lang.All() {
		if s, ok := claude.Section(report, w(v)); ok {
			return s, true
		}
	}
	return "", false
}

// is reports whether s is the word w of any language, ignoring case.
func (w word) is(s string) bool {
	return slices.ContainsFunc(lang.All(), func(v lang.Vocab) bool { return strings.EqualFold(s, w(v)) })
}

// isNone is true for the point of an empty list, "none" or "keine".
func isNone(p string) bool {
	return slices.ContainsFunc(lang.All(), func(v lang.Vocab) bool { return p == v.None })
}

// The files of the evidence folder that lazydev writes or reads itself.
const (
	evidenceReport = "report.md"
	evidencePR     = "pr.md"
)

// claudeStep runs implement, verify or review in the worktree and checks the outcome: after
// implement the report and the status line of the PRD, after verify the report and its evidence,
// after review a clean worktree and with merge: pr the description of the pull request.
func claudeStep(ctx context.Context, t Target, prdName string, r *state.Run, step string, log *logger) (int, error) {
	args := ""
	if step == state.StepImplement || step == state.StepVerify {
		args = r.PRD
	}
	evidence := state.EvidenceDir(t.ID, prdName)
	if usesEvidence(step) {
		// Existing evidence stays, verify overwrites it.
		if err := os.MkdirAll(evidence, 0o755); err != nil {
			return 1, err
		}
	}
	s, exitCode, err := runClaude(ctx, t, prdName, r, step, args, "", log)
	if err != nil {
		return exitCode, err
	}
	if err := checkSummary(s, exitCode); err != nil {
		return exitCode, err
	}
	switch step {
	case state.StepImplement:
		if err := checkReport(s.Result, step, resultImplemented, headingOpenQuestions); err != nil {
			return 0, err
		}
		d, err := project.Read(r.Worktree, filepath.Join(r.Worktree, r.PRD))
		if err != nil {
			return 0, err
		}
		if d.Status.Kind != project.StatusImplemented {
			cfg, _ := t.Repo()
			return 0, fmt.Errorf("status line of %s is %q instead of „%s (date)“", r.PRD, d.Status.Value, lang.For(cfg.Lang()).Implemented)
		}
	case state.StepVerify:
		if err := os.WriteFile(filepath.Join(evidence, evidenceReport), []byte(s.Result), 0o644); err != nil {
			return 0, err
		}
		if err := checkVerify(s.Result, evidence); err != nil {
			return 0, err
		}
	default:
		if err := requireClean(r.Worktree, "worktree after the review"); err != nil {
			return 0, err
		}
		cfg, err := t.Repo()
		if err != nil {
			return 0, err
		}
		if cfg.MergeMode() == repo.MergePR {
			if _, _, err := readPullRequestFile(filepath.Join(evidence, evidencePR)); err != nil {
				return 0, err
			}
		}
	}
	return 0, checkLazydevUntouched(r, r.Base)
}

// usesEvidence is true for the steps that get the evidence folder.
func usesEvidence(step string) bool {
	return step == state.StepVerify || step == state.StepReview
}

// checkVerify checks the report of verify: verified, at least one file under "## Evidence" and each
// of them in the evidence folder dir.
func checkVerify(report, dir string) error {
	if err := checkReport(report, state.StepVerify, resultVerified, headingOpenQuestions); err != nil {
		var b *blockedError
		if errors.As(err, &b) || err.Error() == state.StepVerify+" aborted" {
			return err
		}
		return fmt.Errorf("verify: %w", err)
	}
	section, _ := reportSection(report, headingEvidence)
	files := evidenceFiles(section, dir)
	if len(files) == 0 {
		return errors.New("verify: no evidence under „## Evidence“")
	}
	for _, f := range files {
		if !fsutil.Exists(filepath.Join(dir, f)) {
			return fmt.Errorf("verify: evidence file %s missing in %s", f, dir)
		}
	}
	return nil
}

// evidenceFiles reads the file names of the lines "- <file>: <what it shows>" under "## Evidence".
// Only items at the start of a line count: indented lines, sub-lists included, belong to the item
// above. Backticks and a leading path of the evidence folder dir go.
func evidenceFiles(section, dir string) []string {
	var files []string
	for line := range strings.SplitSeq(section, "\n") {
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		p := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if p == "" || isNone(p) {
			continue
		}
		name, _, _ := strings.Cut(p, ": ")
		name = strings.Trim(strings.TrimSpace(name), "`")
		name = strings.TrimPrefix(name, dir+"/")
		if name != "" {
			files = append(files, name)
		}
	}
	return files
}

// runClaude builds the prompt of the step and starts claude in the worktree. base replaces the
// base branch of the run as $BASE, empty keeps it. Session, model and cost go into the last step as
// they come. Several calls in one step add up their cost.
func runClaude(ctx context.Context, t Target, prdName string, r *state.Run, step, args, base string, log *logger) (claude.Summary, int, error) {
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
	if base == "" {
		base = r.Base
	}
	v := prompts.Vars{Arguments: args, Base: base, Branch: r.Branch}
	var addDirs []string
	if usesEvidence(step) {
		v.Evidence, v.Port = state.EvidenceDir(t.ID, prdName), r.Port
		addDirs = []string{v.Evidence}
	}
	prompt, err := prompts.Build(t.Dir, r.Worktree, cfg, step, v)
	if err != nil {
		return claude.Summary{}, 1, err
	}
	return callClaude(ctx, t, prdName, r, r.Worktree, prompt, tools, mode, addDirs, cfg.Lang(), log)
}

// callClaude runs claude -p with prompt in dir, addDirs as further folders Claude may work in, l
// the language of the built-in agents. Session, model and cost go into the last step as they come.
func callClaude(ctx context.Context, t Target, prdName string, r *state.Run, dir, prompt string, tools []string, mode string, addDirs []string, l lang.Lang, log *logger) (claude.Summary, int, error) {
	baseCost := 0.0
	if c := r.LastStep().CostUSD; c != nil {
		baseCost = *c
	}
	o := claude.Options{
		Dir:            dir,
		Prompt:         prompt,
		AllowedTools:   tools,
		PermissionMode: mode,
		AddDirs:        addDirs,
		Lang:           l,
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
	// resolve is true when the step resolve answers the questions before the run stops.
	resolve bool
}

func (e *blockedError) Error() string {
	return fmt.Sprintf("%s blocked: %s", e.step, e.points[0])
}

// checkReport checks "## Result" in the report of a step, in any language. If it is aborted with
// points in the section questions, the error is a *blockedError.
func checkReport(report, step string, want, questions word) error {
	res, ok := reportSection(report, headingResult)
	if !ok {
		return fmt.Errorf("report of %s without „## Result“", step)
	}
	v := strings.ToLower(claude.FirstLine(res))
	switch {
	case want.is(v):
		return nil
	case resultAborted.is(v):
		q, _ := reportSection(report, questions)
		if points := reportPoints(q); len(points) > 0 {
			return &blockedError{step: step, points: points}
		}
		return fmt.Errorf("%s aborted", step)
	default:
		return fmt.Errorf("unknown result %q in the report of %s", v, step)
	}
}

// reportPoints are the lines of a report section without "- ", empty lines and "none" or „keine“.
func reportPoints(section string) []string {
	var points []string
	for line := range strings.SplitSeq(section, "\n") {
		p := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if p != "" && !isNone(p) {
			points = append(points, p)
		}
	}
	return points
}

// checkLazydevUntouched makes sure Claude changed nothing under .lazydev/ in the worktree, neither
// in commits since base nor uncommitted. base is the base branch, or origin/<base> after the rebase
// of a pull request.
func checkLazydevUntouched(r *state.Run, base string) error {
	committed, err := git.Run(r.Worktree, "diff", "--name-only", base+"...HEAD", "--", repo.Dir)
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
