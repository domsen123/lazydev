package runner

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/hooks"
	"lazydev/internal/prdtemplate"
	"lazydev/internal/project"
	"lazydev/internal/runlog"
	"lazydev/internal/state"
	"lazydev/internal/usage"
)

// Werte unter "## Ergebnis" im Bericht von /implement-prd.
const (
	resultImplemented = "umgesetzt"
	resultAborted     = "abgebrochen"
)

// reviewPrompt ist der Prompt des Schritts review. Seine erste Zeile nennt ihn im Log /arewedone.
// $BASE steht für den Ziel-Branch.
//
//go:embed prompts/arewedone.md
var reviewPrompt string

func prompt(r *state.Run, step string) string {
	if step == StepImplement {
		return prdtemplate.Get(prdTemplate(r)).ImplementPrompt(r.PRD)
	}
	return strings.ReplaceAll(reviewPrompt, "$BASE", r.Base)
}

// prdTemplate liest das Template aus der Statuszeile des PRDs im Worktree. Ist das PRD nicht
// lesbar, gilt das Standard-Template. Den Fehler meldet dann der Schritt selbst.
func prdTemplate(r *state.Run) string {
	f, err := os.Open(filepath.Join(r.Worktree, r.PRD))
	if err != nil {
		return ""
	}
	defer f.Close()
	_, status, _ := project.ParseHeader(f)
	return prdtemplate.Of(status)
}

func allowedTools(p config.Project) []string {
	return append(append([]string{}, claude.DefaultAllowedTools...), p.AllowedTools...)
}

// claudeStep startet claude im Worktree und prüft danach das Ergebnis des Schritts.
func claudeStep(ctx context.Context, cfg *config.Config, t Target, prdName string, r *state.Run, step string, log *runlog.Logger) (int, error) {
	s, exitCode, err := runClaude(ctx, cfg, t, prdName, r, prompt(r, step), log)
	if err != nil {
		return exitCode, err
	}
	if err := checkSummary(s, exitCode); err != nil {
		return exitCode, err
	}
	if step == StepImplement {
		if err := checkReport(s.Result, "/implement-prd", resultImplemented, "Offene Fragen"); err != nil {
			return 0, err
		}
		return 0, checkImplemented(r)
	}
	return 0, checkReviewed(t, r, log)
}

// runClaude setzt erzeugte Dateien zurück und startet claude im Worktree. Sitzung und Kosten
// schreibt runClaude laufend in den letzten Schritt. Mehrere Aufrufe in einem Schritt addieren
// ihre Kosten.
func runClaude(ctx context.Context, cfg *config.Config, t Target, prdName string, r *state.Run, prompt string, log *runlog.Logger) (claude.Summary, int, error) {
	if err := hooks.For(t.Config).Clean(env(t, r, log)); err != nil {
		return claude.Summary{}, 1, err
	}
	baseCost := 0.0
	if c := r.LastStep().CostUSD; c != nil {
		baseCost = *c
	}
	o := claude.Options{
		Binary:       cfg.Claude.Binary,
		Dir:          r.Worktree,
		Prompt:       prompt,
		AllowedTools: allowedTools(t.Config),
		Log:          eventLog{log},
		OnChange: func(s claude.Summary) {
			last := r.LastStep()
			last.SessionID = s.SessionID
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

// checkReport prüft "## Ergebnis" im Bericht eines Commands. Weicht der Wert von want ab, nennt
// der Fehler den ersten Punkt aus dem Abschnitt questions.
func checkReport(report, command, want, questions string) error {
	res, ok := claude.Section(report, "Ergebnis")
	if !ok {
		return fmt.Errorf("Bericht von %s ohne Abschnitt „## Ergebnis“", command)
	}
	switch v := strings.ToLower(claude.FirstLine(res)); v {
	case want:
		return nil
	case resultAborted:
		q, _ := claude.Section(report, questions)
		if q := claude.FirstLine(q); q != "" && q != "keine" && q != "- keine" {
			return fmt.Errorf("%s abgebrochen: %s", command, strings.TrimPrefix(q, "- "))
		}
		return fmt.Errorf("%s abgebrochen", command)
	default:
		return fmt.Errorf("unbekanntes Ergebnis %q im Bericht von %s", v, command)
	}
}

// checkImplemented prüft, dass /implement-prd die Statuszeile auf "umgesetzt" gesetzt hat.
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

// checkReviewed prüft, dass /arewedone alles committet hat. Der Merge braucht einen sauberen
// Worktree.
func checkReviewed(t Target, r *state.Run, log *runlog.Logger) error {
	if err := hooks.For(t.Config).Clean(env(t, r, log)); err != nil {
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
