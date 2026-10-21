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
	"lazydev/internal/project"
	"lazydev/internal/prompts"
	"lazydev/internal/runlog"
	"lazydev/internal/state"
	"lazydev/internal/usage"
)

// Werte unter "## Ergebnis" im Bericht des Schritts implement.
const (
	resultImplemented = "umgesetzt"
	resultAborted     = "abgebrochen"
)

// stepPrompts ordnet den Claude-Schritten des Runners ihren Prompt zu.
var stepPrompts = map[string]prompts.Step{
	StepImplement: prompts.Implement,
	StepReview:    prompts.Review,
}

// allowedTools ergänzt die Standard-Freigaben um allowedTools des Projekts.
func allowedTools(r config.Repo) []string {
	return append(slices.Clone(claude.DefaultAllowedTools), r.AllowedTools...)
}

// claudeStep startet claude im Worktree und prüft danach das Ergebnis des Schritts.
func claudeStep(ctx context.Context, cfg *config.Config, t Target, prdName string, r *state.Run, step string, log *runlog.Logger) (int, error) {
	args := ""
	if step == StepImplement {
		args = r.PRD
	}
	s, exitCode, err := runClaude(ctx, cfg, t, prdName, r, stepPrompts[step], args, log)
	if err != nil {
		return exitCode, err
	}
	if err := checkSummary(s, exitCode); err != nil {
		return exitCode, err
	}
	if step == StepImplement {
		if err := checkReport(s.Result, string(prompts.Implement), resultImplemented, "Offene Fragen"); err != nil {
			return 0, err
		}
		if err := checkImplemented(r); err != nil {
			return 0, err
		}
	} else if err := checkReviewed(t, r, log); err != nil {
		return 0, err
	}
	return 0, checkLazydevUntouched(r)
}

// runClaude baut den Prompt des Schritts, setzt erzeugte Dateien zurück und startet claude im
// Worktree. Fehlt eine Knowledge-Datei, startet claude nicht. Sitzung, Modell und Kosten schreibt
// runClaude laufend in den letzten Schritt. Mehrere Aufrufe in einem Schritt addieren ihre Kosten.
func runClaude(ctx context.Context, cfg *config.Config, t Target, prdName string, r *state.Run, step prompts.Step, args string, log *runlog.Logger) (claude.Summary, int, error) {
	repo, err := t.Repo()
	if err != nil {
		return claude.Summary{}, 1, err
	}
	p, err := prompts.Build(t.Dir(), repo, step, prompts.Vars{Arguments: args, Base: r.Base, Branch: r.Branch, WorkDir: r.Worktree})
	if err != nil {
		return claude.Summary{}, 1, err
	}
	h, err := t.Hooks()
	if err != nil {
		return claude.Summary{}, 1, err
	}
	if err := h.Clean(env(t, r, log)); err != nil {
		return claude.Summary{}, 1, err
	}
	baseCost := 0.0
	if c := r.LastStep().CostUSD; c != nil {
		baseCost = *c
	}
	o := claude.Options{
		Binary:       cfg.Claude.Binary,
		Dir:          r.Worktree,
		Prompt:       p.Text,
		AllowedTools: allowedTools(repo),
		Log:          eventLog{log},
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
			// Die Sitzung soll im Zustand stehen, bevor der Schritt endet, damit sie sich
			// während des Laufs öffnen lässt.
			if err := state.Save(t.ID, prdName, r); err != nil {
				log.Printf("Zustand nicht gespeichert: %v", err)
			}
		},
	}
	log.Printf("Prompt: %s", p.Source)
	if len(p.Knowledge) > 0 {
		log.Printf("Knowledge: %s", strings.Join(p.Knowledge, ", "))
	}
	log.Printf("$ %s %s", o.Binary, claude.FirstLine(o.Prompt))
	s, exitCode, err := claude.Run(ctx, o)
	if err != nil {
		return s, exitCode, fmt.Errorf("%s: %w", o.Binary, err)
	}
	return s, exitCode, nil
}

// checkSummary wertet Exit-Code und letztes result-Ereignis aus.
func checkSummary(s claude.Summary, exitCode int) error {
	switch {
	case exitCode != 0:
		if s.Result != "" {
			return fmt.Errorf("claude endete mit Exit-Code %d: %s", exitCode, claude.FirstLine(s.Result))
		}
		return fmt.Errorf("claude endete mit Exit-Code %d", exitCode)
	case !s.HasResult:
		return errors.New("claude endete ohne result-Ereignis")
	case s.IsError:
		return fmt.Errorf("claude meldet einen Fehler: %s", claude.FirstLine(s.Result))
	}
	return nil
}

// checkReport prüft "## Ergebnis" im Bericht eines Schritts. Weicht der Wert von want ab, nennt
// der Fehler den ersten Punkt aus dem Abschnitt questions.
func checkReport(report, step, want, questions string) error {
	res, ok := claude.Section(report, "Ergebnis")
	if !ok {
		return fmt.Errorf("Bericht von %s ohne Abschnitt „## Ergebnis“", step)
	}
	switch v := strings.ToLower(claude.FirstLine(res)); v {
	case want:
		return nil
	case resultAborted:
		q, _ := claude.Section(report, questions)
		if q := claude.FirstLine(q); q != "" && q != "keine" && q != "- keine" {
			return fmt.Errorf("%s abgebrochen: %s", step, strings.TrimPrefix(q, "- "))
		}
		return fmt.Errorf("%s abgebrochen", step)
	default:
		return fmt.Errorf("unbekanntes Ergebnis %q im Bericht von %s", v, step)
	}
}

// checkImplemented prüft, dass implement die Statuszeile auf "umgesetzt" gesetzt hat.
func checkImplemented(r *state.Run) error {
	f, err := os.Open(filepath.Join(r.Worktree, r.PRD))
	if err != nil {
		return err
	}
	defer f.Close()
	_, status, err := project.ParseHeader(f)
	if err != nil {
		return err
	}
	if status.Kind != project.StatusImplemented {
		return fmt.Errorf("Statuszeile von %s steht auf %q statt „umgesetzt (Datum)“", r.PRD, status.Value)
	}
	return nil
}

// checkReviewed prüft, dass review alles committet hat. Der Merge braucht einen sauberen
// Worktree.
func checkReviewed(t Target, r *state.Run, log *runlog.Logger) error {
	h, err := t.Hooks()
	if err != nil {
		return err
	}
	if err := h.Clean(env(t, r, log)); err != nil {
		return err
	}
	out, err := git.Run(r.Worktree, "status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty := git.Lines(out); len(dirty) > 0 {
		return fmt.Errorf("Worktree nach dem Review nicht sauber: %s", strings.Join(dirty, ", "))
	}
	return nil
}

// checkLazydevUntouched prüft, dass Claude im Worktree nichts unter .lazydev/ geändert hat, weder
// in Commits seit dem Ziel-Branch noch uncommittet.
func checkLazydevUntouched(r *state.Run) error {
	committed, err := git.Run(r.Worktree, "diff", "--name-only", r.Base+"...HEAD", "--", config.RepoDir)
	if err != nil {
		return err
	}
	status, err := git.Run(r.Worktree, "status", "--porcelain", "--untracked-files=all", "--", config.RepoDir)
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
		return fmt.Errorf("%s/ im Worktree geändert: %s", config.RepoDir, strings.Join(changed, ", "))
	}
	return nil
}

// eventLog schreibt stream-json unverändert ins Log und eine Kurzfassung nach echo.
type eventLog struct {
	log *runlog.Logger
}

func (w eventLog) Write(line []byte) (int, error) {
	w.log.Append(claude.WithTimestamp(line))
	usage.Record(line)
	for _, d := range claude.Describe(line) {
		w.log.Echo(d)
	}
	return len(line), nil
}
