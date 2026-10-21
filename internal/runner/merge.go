package runner

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"lazydev/internal/config"
	"lazydev/internal/devserver"
	"lazydev/internal/git"
	"lazydev/internal/hooks"
	"lazydev/internal/project"
	"lazydev/internal/runlog"
	"lazydev/internal/state"
	"lazydev/internal/worktree"
)

// mergePrompt löst Konflikte beim Rebase. $ARGUMENTS steht für Branch und Fehlerkontext, $BASE für
// den Ziel-Branch. Die erste Zeile nennt den Schritt im Log /merge-prd.
//
//go:embed prompts/merge.md
var mergePrompt string

// resultMergeReady ist der Erfolgswert unter "## Ergebnis" im Bericht von /merge-prd.
const resultMergeReady = "bereit für fast-forward"

// mergePollInterval ist der Abstand, in dem ein PRD in der Merge-Queue auf die Sperre oder einen
// sauberen Hauptordner prüft. Tests verkürzen ihn.
var mergePollInterval = 5 * time.Second

// NoteMainDirty steht im Zustand, solange der Hauptordner den Fast-Forward blockiert.
const NoteMainDirty = "Hauptordner nicht sauber"

func mergeLock(projectID string) string {
	return projectID + ".merge"
}

// mergeStep bringt den Branch per Rebase auf den Ziel-Branch, prüft ihn und führt den Fast-Forward im
// Hauptordner aus. Pro Projekt mergt immer nur ein Runner, die anderen warten in der
// Merge-Queue.
func mergeStep(ctx context.Context, cfg *config.Config, t Target, prdName string, r *state.Run, log *runlog.Logger) (int, error) {
	unlock, err := waitMergeLock(ctx, t.ID, log)
	if err != nil {
		return 1, err
	}
	defer unlock()
	r.Phase = project.PhaseMerge
	r.Note = ""
	if err := state.Save(t.ID, prdName, r); err != nil {
		return 1, err
	}
	h := hooks.For(t.Config)
	e := env(t, r, log)
	m := merger{ctx: ctx, cfg: cfg, t: t, prdName: prdName, r: r, log: log, h: h, e: e}
	if err := m.run(); err != nil {
		return 1, err
	}
	return 0, nil
}

type merger struct {
	ctx     context.Context
	cfg     *config.Config
	t       Target
	prdName string
	r       *state.Run
	log     *runlog.Logger
	h       hooks.Hooks
	e       hooks.Env
}

func (m merger) run() error {
	wt, main, base := m.r.Worktree, m.t.Dir(), m.r.Base
	if pid := devserver.PID(m.t.ID, m.prdName); pid != 0 {
		m.log.Printf("Stoppe den Dev-Server im Worktree (PID %d)", pid)
		if err := devserver.Stop(m.t.ID, m.prdName); err != nil {
			return err
		}
	}
	if err := m.h.Clean(m.e); err != nil {
		return err
	}
	if err := requireClean(wt, "Worktree"); err != nil {
		return err
	}
	oldBase, err := git.Run(wt, "merge-base", base, "HEAD")
	if err != nil {
		return err
	}

	m.log.Printf("$ git rebase %s", base)
	if _, err := git.Run(wt, "rebase", base); err != nil {
		m.log.Printf("Konflikt beim Rebase: %v", err)
		if _, err := git.Run(wt, "rebase", "--abort"); err != nil {
			m.log.Printf("git rebase --abort: %v", err)
		}
		if err := m.mergePRD(""); err != nil {
			return err
		}
	}
	if err := m.resetDatabaseIfMigrated(oldBase, base); err != nil {
		return err
	}

	if err := m.h.Check(m.e); err != nil {
		m.log.Printf("Prüfung rot: %v", err)
		head, _ := git.Run(wt, "rev-parse", "HEAD")
		if err := m.mergePRD("Prüfung nach dem Rebase fehlgeschlagen: " + err.Error()); err != nil {
			return err
		}
		if err := m.resetDatabaseIfMigrated(head, "HEAD"); err != nil {
			return err
		}
		if err := m.h.Check(m.e); err != nil {
			return fmt.Errorf("Prüfung nach /merge-prd weiter rot: %w", err)
		}
	}

	if err := m.recordRunInfo(); err != nil {
		return err
	}
	if err := m.waitMainClean(); err != nil {
		return err
	}
	mainBefore, err := git.Run(main, "rev-parse", base)
	if err != nil {
		return err
	}
	m.log.Printf("$ git merge --ff-only %s", m.r.Branch)
	if _, err := git.Run(main, "merge", "--ff-only", m.r.Branch); err != nil {
		return err
	}
	m.log.Printf("%s steht auf %s", base, m.r.Branch)

	var notes []string
	if m.h.MigrationsDir != "" {
		if changed, _ := changedFiles(main, mainBefore, base, m.h.MigrationsDir); len(changed) > 0 {
			notes = append(notes, "Neue Migrationen: pnpm db:migrate im Hauptordner ausführen")
		}
	}
	if err := m.cleanup(); err != nil {
		m.log.Printf("Aufräumen unvollständig: %v", err)
		notes = append(notes, "Aufräumen unvollständig: "+err.Error())
	}
	m.r.Note = strings.Join(notes, ". ")
	return nil
}

// mergePRD lässt /merge-prd den Branch auf den Ziel-Branch bringen. context ist der Fehlerkontext.
func (m merger) mergePRD(context string) error {
	args := m.r.Branch
	if context != "" {
		args += " " + context
	}
	prompt := strings.NewReplacer("$ARGUMENTS", args, "$BASE", m.r.Base).Replace(mergePrompt)
	s, exitCode, err := runClaude(m.ctx, m.cfg, m.t, m.prdName, m.r, prompt, m.log)
	if err == nil {
		err = checkSummary(s, exitCode)
	}
	if err == nil {
		err = checkReport(s.Result, "/merge-prd", resultMergeReady, "Offene Punkte")
	}
	if err != nil {
		return err
	}
	// Der Bericht allein reicht nicht: Der Branch muss wirklich auf dem Ziel-Branch stehen.
	wt, base := m.r.Worktree, m.r.Base
	if _, err := git.Run(wt, "merge-base", "--is-ancestor", base, "HEAD"); err != nil {
		return fmt.Errorf("nach /merge-prd steht %s nicht auf %s", m.r.Branch, base)
	}
	if err := m.h.Clean(m.e); err != nil {
		return err
	}
	return requireClean(wt, "Worktree nach /merge-prd")
}

// recordRunInfo schreibt Kosten und Modell des Laufs in die Statuszeile des PRDs im Worktree und
// committet sie ohne Claude. Die Prüfung läuft danach nicht erneut, der Commit ändert nur das PRD.
// Ändern sich die Werte nicht, entsteht kein Commit.
func (m merger) recordRunInfo() error {
	path := filepath.Join(m.r.Worktree, m.r.PRD)
	cost, model := m.r.Cost(), m.r.ImplementModel()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Kosten nicht im PRD vermerkt: %w", err)
	}
	updated := project.SetRunInfo(string(data), cost, model)
	// Ohne Statuszeile liefert SetRunInfo den Inhalt unverändert. Erst das Feld „Kosten:“ im
	// Ergebnis zeigt, dass die Statuszeile da ist.
	_, status, err := project.ParseHeader(strings.NewReader(updated))
	if _, ok, _ := project.RunInfo(status); err == nil && !ok {
		err = fmt.Errorf("keine Statuszeile in %s", m.r.PRD)
	}
	if err != nil {
		return fmt.Errorf("Kosten nicht im PRD vermerkt: %w", err)
	}
	info := strconv.FormatFloat(cost, 'f', 2, 64) + " USD"
	if model != "" {
		info += " · " + model
	}
	changed := updated != string(data)
	if changed {
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("Kosten nicht im PRD vermerkt: %w", err)
		}
	}
	m.log.Printf("Kosten im PRD vermerkt: %s", info)
	if !changed {
		return nil
	}
	msg := "docs(prd): record cost of " + m.prdName
	if _, err := git.Run(m.r.Worktree, "commit", "-m", msg, "--", m.r.PRD); err != nil {
		// Ohne den alten Inhalt bliebe der Worktree schmutzig und der nächste Versuch scheiterte daran.
		if werr := os.WriteFile(path, data, 0o644); werr != nil {
			m.log.Printf("PRD nicht zurückgesetzt: %v", werr)
		}
		return err
	}
	m.log.Printf("Commit: %s", msg)
	return nil
}

// resetDatabaseIfMigrated legt die Worktree-Datenbanken neu an, wenn sich Migrationen zwischen
// from und to geändert haben.
func (m merger) resetDatabaseIfMigrated(from, to string) error {
	if m.h.MigrationsDir == "" {
		return nil
	}
	changed, err := changedFiles(m.r.Worktree, from, to, m.h.MigrationsDir)
	if err != nil || len(changed) == 0 {
		return err
	}
	m.log.Printf("Migrationen geändert (%d Dateien), lege die Worktree-Datenbanken neu an", len(changed))
	return m.h.ResetDatabase(m.e)
}

// waitMainClean wartet, bis der Hauptordner auf dem Ziel-Branch steht und sauber ist. Ungetrackte PRDs und
// die Dateien aus IgnoreDirty stören nicht.
func (m merger) waitMainClean() error {
	waiting := false
	for {
		problem, err := mainDirProblem(m.t.Dir(), m.r.Base, m.h.IgnoreDirty)
		if err != nil {
			return err
		}
		if problem == "" {
			if waiting {
				m.r.Note = ""
				return state.Save(m.t.ID, m.prdName, m.r)
			}
			return nil
		}
		if !waiting {
			m.log.Printf("%s: %s. Der Merge wartet.", NoteMainDirty, problem)
			m.r.Phase = project.PhaseMergeQueue
			m.r.Note = NoteMainDirty + ": " + problem
			if err := state.Save(m.t.ID, m.prdName, m.r); err != nil {
				return err
			}
			waiting = true
		}
		if err := sleep(m.ctx, mergePollInterval); err != nil {
			return err
		}
	}
}

// mainDirProblem beschreibt, was den Fast-Forward im Hauptordner blockiert. Leer heißt: nichts.
func mainDirProblem(dir, base string, ignore []string) (string, error) {
	branch, err := git.Run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if branch != base {
		return fmt.Sprintf("Branch %s statt %s", branch, base), nil
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
		if strings.HasPrefix(line, "?? ") && strings.HasPrefix(path, project.PRDDir+"/") {
			continue
		}
		if slices.Contains(ignore, path) {
			continue
		}
		dirty = append(dirty, path)
	}
	if len(dirty) > 0 {
		return "geändert: " + strings.Join(dirty, ", "), nil
	}
	return "", nil
}

// cleanup entfernt nach dem Fast-Forward Datenbanken, Worktree und Branch. Der Branch ist
// gemergt, git branch -d reicht also.
func (m merger) cleanup() error {
	var errs []error
	if err := m.h.Teardown(m.e); err != nil {
		errs = append(errs, err)
	}
	m.log.Printf("Entferne Worktree %s und Branch %s", m.r.Worktree, m.r.Branch)
	if err := worktree.Remove(m.t.Dir(), m.r.Worktree, m.r.Branch, false); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// waitMergeLock wartet auf den Merge-Slot des Projekts. Ein Abbruch über ctx beendet das Warten.
func waitMergeLock(ctx context.Context, projectID string, log *runlog.Logger) (func(), error) {
	logged := false
	for {
		unlock, ok, err := state.TryLock(mergeLock(projectID))
		if err != nil || ok {
			return unlock, err
		}
		if !logged {
			log.Printf("Ein anderes PRD mergt gerade, warte auf den Merge-Slot")
			logged = true
		}
		if err := sleep(ctx, mergePollInterval); err != nil {
			return nil, err
		}
	}
}

func requireClean(dir, what string) error {
	out, err := git.Run(dir, "status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty := git.Lines(out); len(dirty) > 0 {
		return fmt.Errorf("%s nicht sauber: %s", what, strings.Join(dirty, ", "))
	}
	return nil
}

func changedFiles(dir, from, to, path string) ([]string, error) {
	out, err := git.Run(dir, "diff", "--name-only", from, to, "--", path)
	if err != nil {
		return nil, err
	}
	return git.Lines(out), nil
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
