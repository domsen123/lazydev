package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// spawnRunner startet "lazydev run" losgelöst in einer eigenen Sitzung und liefert die PID.
// Tests ersetzen die Funktion.
var spawnRunner = func(projectID, prdName string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	out, err := os.OpenFile(state.RunnerLogPath(projectID, prdName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	cmd := exec.Command(exe, "run", projectID, prdName)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	return pid, cmd.Process.Release()
}

// isRunning meldet, ob der Runner eines Laufs lebt. Tests ersetzen die Funktion.
var isRunning = func(r *state.Run) bool { return r.Running() }

// Start startet den Runner eines PRDs losgelöst. Ist maxParallel erreicht, bleibt das PRD in
// Phase "wartet", und started ist false. Ein Runner, der endet, startet wartende PRDs.
func Start(cfg *config.Config, t Target, prdName string) (started bool, err error) {
	if err := checkClaude(cfg); err != nil {
		return false, err
	}
	if err := CheckTrust(t); err != nil {
		return false, err
	}
	unlock, err := state.Lock(startLock)
	if err != nil {
		return false, err
	}
	defer unlock()
	r, err := prepare(cfg, t, prdName)
	if err != nil {
		return false, err
	}
	entries, err := state.All()
	if err != nil {
		return false, err
	}
	if countRunning(entries) >= cfg.MaxParallel {
		r.Phase = project.PhaseWaiting
		r.Error = nil
		return false, state.Save(t.ID, prdName, r)
	}
	return true, spawn(t.ID, prdName, r)
}

// Merge gibt den Merge eines PRDs frei, wenn autoMerge aus ist, und startet den Runner.
func Merge(cfg *config.Config, t Target, prdName string) (started bool, err error) {
	r, err := state.Load(t.ID, prdName)
	if err != nil {
		return false, err
	}
	if NextStep(r) != StepMerge {
		return false, fmt.Errorf("%s ist nicht bereit für den Merge, Phase %s", prdName, r.Effective().Label())
	}
	r.MergeApproved = true
	r.Note = ""
	if err := state.Save(t.ID, prdName, r); err != nil {
		return false, err
	}
	return Start(cfg, t, prdName)
}

// StartWaiting startet wartende PRDs in der Reihenfolge ihres Starts, bis maxParallel erreicht
// ist. PRDs von Projekten, die nicht mehr eingetragen sind, überspringt StartWaiting.
func StartWaiting(cfg *config.Config) error {
	unlock, err := state.Lock(startLock)
	if err != nil {
		return err
	}
	defer unlock()
	entries, err := state.All()
	if err != nil {
		return err
	}
	running := countRunning(entries)
	var errs []error
	for _, e := range entries {
		if running >= cfg.MaxParallel {
			break
		}
		if e.Run.Phase != project.PhaseWaiting || isRunning(e.Run) {
			continue
		}
		if _, err := FindProject(cfg, e.ProjectID); err != nil {
			continue
		}
		if err := spawn(e.ProjectID, e.PRDName, e.Run); err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", e.ProjectID, e.PRDName, err))
			continue
		}
		running++
	}
	return errors.Join(errs...)
}

// spawn speichert die PID, solange die Startsperre gilt. Der neue Runner wartet auf die
// Sperre und überschreibt den Zustand erst danach.
func spawn(projectID, prdName string, r *state.Run) error {
	pid, err := spawnRunner(projectID, prdName)
	if err != nil {
		return err
	}
	r.PID = pid
	return state.Save(projectID, prdName, r)
}

func countRunning(entries []state.Entry) int {
	n := 0
	for _, e := range entries {
		if isRunning(e.Run) {
			n++
		}
	}
	return n
}

// Stop sendet SIGTERM an die Prozessgruppe des Runners. Der Runner markiert das PRD dann als
// "abgebrochen". Ein wartendes PRD ohne Runner setzt Stop direkt auf "abgebrochen".
func Stop(t Target, prdName string) error {
	r, err := state.Load(t.ID, prdName)
	if err != nil {
		return err
	}
	if !isRunning(r) {
		if r.Phase == project.PhaseWaiting {
			r.Phase = project.PhaseAborted
			return state.Save(t.ID, prdName, r)
		}
		return fmt.Errorf("für %s läuft kein Runner, Phase %s", prdName, r.Effective().Label())
	}
	// Ein losgelöster Runner leitet seine Sitzung, die PID ist also auch die Gruppen-ID. Ein
	// Runner aus dem Terminal gehört zur Gruppe der Shell und bekommt das Signal allein.
	if err := syscall.Kill(-r.PID, syscall.SIGTERM); err == nil {
		return nil
	}
	return syscall.Kill(r.PID, syscall.SIGTERM)
}
