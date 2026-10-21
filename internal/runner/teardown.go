package runner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"lazydev/internal/devserver"
	"lazydev/internal/project"
	"lazydev/internal/runlog"
	"lazydev/internal/state"
	"lazydev/internal/worktree"
)

// Teardown verwirft den Lauf eines PRDs: Datenbanken, Worktree, Branch und Zustand. Ein PRD,
// das in main fehlt, legt Teardown vorher ungetrackt in den Hauptordner zurück. Teardown
// arbeitet auch ohne Zustand, damit sich ein halb angelegter Lauf aufräumen lässt.
func Teardown(t Target, prdName string, echo io.Writer) error {
	log, err := runlog.Open("", echo)
	if err != nil {
		return err
	}
	r, err := state.Load(t.ID, prdName)
	if errors.Is(err, fs.ErrNotExist) {
		r = &state.Run{
			Branch:   worktree.Branch(prdName),
			Worktree: worktree.Path(t.ID, prdName),
			Slug:     worktree.Slug(prdName),
		}
		err = nil
	}
	if err != nil {
		return err
	}
	if isRunning(r) {
		return fmt.Errorf("für %s läuft noch ein Runner (PID %d). Zum Stoppen: lazydev stop %s %s",
			prdName, r.PID, t.ID, prdName)
	}

	var errs []error
	if err := devserver.Stop(t.ID, prdName); err != nil {
		errs = append(errs, err)
	}
	if h, err := t.Hooks(); err != nil {
		errs = append(errs, err)
	} else if err := h.Teardown(env(t, r, log)); err != nil {
		errs = append(errs, err)
	}
	if err := restorePRD(t.Dir(), r.Worktree, r.PRD, prdName, log); err != nil {
		// Ohne gesichertes PRD bleibt der Worktree stehen, damit nichts verloren geht.
		return errors.Join(append(errs, err)...)
	}
	log.Printf("Entferne Worktree %s und Branch %s", r.Worktree, r.Branch)
	if err := worktree.Remove(t.Dir(), r.Worktree, r.Branch, true); err != nil {
		errs = append(errs, err)
	}
	if err := state.Remove(t.ID, prdName); err != nil {
		errs = append(errs, err)
	}
	if err := devserver.Remove(t.ID, prdName); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// restorePRD legt das PRD aus dem Worktree in den Hauptordner, wenn es dort fehlt. Der Status
// geht zurück auf "geplant", weil die Umsetzung mit dem Worktree verworfen ist.
// rel ist der Pfad aus dem Zustand. Fehlt er, sucht restorePRD das PRD über seinen Namen.
func restorePRD(mainDir, wt, rel, prdName string, log *runlog.Logger) error {
	if rel == "" {
		rel = findPRD(wt, prdName)
	}
	src := filepath.Join(wt, rel)
	dst := filepath.Join(mainDir, rel)
	if rel == "" || !fileExists(src) || fileExists(dst) {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(project.ResetStatus(string(data))), 0o644); err != nil {
		return err
	}
	log.Printf("PRD %s mit Status geplant zurück in den Hauptordner gelegt", rel)
	return nil
}

func findPRD(wt, prdName string) string {
	found := ""
	filepath.WalkDir(filepath.Join(wt, project.PRDDir), func(path string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() && e.Name() == prdName+".md" {
			found, _ = filepath.Rel(wt, path)
			return fs.SkipAll
		}
		return nil
	})
	return found
}
