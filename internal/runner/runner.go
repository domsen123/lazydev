// Package runner works through the steps of a PRD: setup (worktree, PRD, setup hook), implement,
// review and merge (rebase, check hook, fast-forward, cleanup). Each PRD runs in a detached
// process "lazydev run", the TUI only reads its state and log files. Every step must be
// repeatable, a new start continues with the failed step.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/fsutil"
	"lazydev/internal/notify"
	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// MaxParallel limits the runners that work at the same time, the others wait.
const MaxParallel = 3

// startLock guards counting the running runners and starting new ones.
const startLock = "start"

// stepPhases are the phases of the steps. The merge starts in the merge queue and becomes merge
// with the merge slot.
var stepPhases = map[string]project.Phase{
	state.StepSetup:     project.PhaseSetup,
	state.StepImplement: project.PhaseImplement,
	state.StepReview:    project.PhaseReview,
	state.StepMerge:     project.PhaseMergeQueue,
}

// Target is a project of the list with its ID.
type Target struct {
	ID, Dir string
}

// Find looks up a project of the list by ID or path.
func Find(idOrPath string) (Target, error) {
	paths, err := config.LoadProjects()
	if err != nil {
		return Target{}, err
	}
	abs, _ := config.ExpandPath(idOrPath)
	for i, id := range config.IDs(paths) {
		if id == idOrPath || paths[i] == abs {
			return Target{ID: id, Dir: paths[i]}, nil
		}
	}
	return Target{}, fmt.Errorf("no project %q in the list, add it in the TUI first", idOrPath)
}

// Repo reads .lazydev/config.yaml again for every step, so uncommitted changes apply right away.
func (t Target) Repo() (repo.Config, error) {
	info := repo.Read(t.Dir, t.ID)
	return info.Config, info.Err
}

// CheckTrust makes sure hooks and allowed tools of config.yaml are trusted.
func (t Target) CheckTrust() error {
	info := repo.Read(t.Dir, t.ID)
	switch {
	case info.Err != nil:
		return info.Err
	case info.Trust == repo.TrustPending:
		return fmt.Errorf("hooks or allowed tools in %s/config.yaml are not trusted, confirm them with s in the TUI or lazydev trust %s", repo.Dir, t.ID)
	}
	return nil
}

// checkClaude makes sure claude exists before the start, otherwise a run fails after the setup.
func checkClaude() error {
	if _, err := exec.LookPath(claude.Command[0]); err != nil {
		return fmt.Errorf("%s not found, install Claude Code", claude.Command[0])
	}
	return nil
}

// Start starts the runner of a PRD detached. With MaxParallel reached, the PRD waits and started
// is false. A runner that ends starts waiting PRDs.
func Start(t Target, prdName string) (started bool, err error) {
	if err := checkClaude(); err != nil {
		return false, err
	}
	if err := t.CheckTrust(); err != nil {
		return false, err
	}
	unlock, err := state.Lock(startLock)
	if err != nil {
		return false, err
	}
	defer unlock()
	r, err := prepare(t, prdName)
	if err != nil {
		return false, err
	}
	if countRunning(state.All()) >= MaxParallel {
		r.Phase, r.Error = project.PhaseWaiting, nil
		return false, state.Save(t.ID, prdName, r)
	}
	return true, spawn(t.ID, prdName, r)
}

// startWaiting starts waiting PRDs in the order of their start until MaxParallel is reached.
func startWaiting() error {
	unlock, err := state.Lock(startLock)
	if err != nil {
		return err
	}
	defer unlock()
	entries := state.All()
	running := countRunning(entries)
	var errs []error
	for _, e := range entries {
		if running >= MaxParallel {
			break
		}
		if e.Run.Phase != project.PhaseWaiting || e.Run.Running() {
			continue
		}
		if err := spawn(e.ID, e.PRDName, e.Run); err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", e.ID, e.PRDName, err))
			continue
		}
		running++
	}
	return errors.Join(errs...)
}

func countRunning(entries []state.Entry) int {
	n := 0
	for _, e := range entries {
		if e.Run.Running() {
			n++
		}
	}
	return n
}

// spawn starts "lazydev run" in its own session and saves the PID while the start lock holds. The
// new runner waits for the lock and only then overwrites the state.
func spawn(id, prdName string, r *state.Run) error {
	bin, err := binary()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(state.RunsDir(id), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(state.RunnerLogPath(id, prdName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	cmd := exec.Command(bin, "run", id, r.ProjectDir, prdName)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	r.PID = cmd.Process.Pid
	cmd.Process.Release()
	return state.Save(id, prdName, r)
}

// Executable is the binary runners start from. Tests replace it with a built lazydev.
var Executable = os.Executable

// binary copies the own executable to a fixed place and returns it. "go run" deletes its binary
// when it exits, and a runner must still start waiting PRDs after that. A running runner keeps
// the code it started with.
func binary() (string, error) {
	exe, err := Executable()
	if err != nil {
		return "", err
	}
	dst := filepath.Join(state.Dir(), "bin", state.BinaryName)
	if exe == dst {
		return dst, nil
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return "", err
	}
	return dst, fsutil.WriteAtomic(dst, data, 0o755)
}

// Stop sends SIGTERM to the runner, which marks the PRD as aborted and ends its process group. A
// waiting PRD without runner becomes aborted right away.
func Stop(id, prdName string) error {
	r, err := state.Load(id, prdName)
	if err != nil {
		return err
	}
	if !r.Running() {
		if r.Phase == project.PhaseWaiting {
			r.Phase = project.PhaseAborted
			return state.Save(id, prdName, r)
		}
		return fmt.Errorf("no runner for %s, phase %s", prdName, r.Effective())
	}
	return syscall.Kill(r.PID, syscall.SIGTERM)
}

// RunMain is "lazydev run <id> <dir> <prd>": it works through the open steps in the foreground and
// starts waiting PRDs when it ends.
func RunMain(id, dir, prdName string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigs
		// The context ends first, so the step sees the abort and not just a killed Claude.
		// Then the signal reaches the process group: Claude, hooks and their children. A detached
		// runner leads its session, so its PID is the group ID. The runner catches the signal
		// again and ignores it.
		cancel()
		if syscall.Getpgrp() == os.Getpid() {
			syscall.Kill(-os.Getpid(), syscall.SIGTERM)
		}
	}()
	err := Run(ctx, Target{ID: id, Dir: dir}, prdName)
	if nextErr := startWaiting(); nextErr != nil {
		fmt.Fprintln(os.Stderr, "waiting PRDs not started:", nextErr)
	}
	return err
}

// Run works through the open steps of a PRD until it is merged, ctx ends or a step fails. It does
// not honor MaxParallel, Start does.
func Run(ctx context.Context, t Target, prdName string) error {
	if err := checkClaude(); err != nil {
		return err
	}
	if err := t.CheckTrust(); err != nil {
		return err
	}
	unlock, err := state.Lock(startLock)
	if err != nil {
		return err
	}
	r, err := prepare(t, prdName)
	if err == nil {
		r.PID, r.Error = os.Getpid(), nil
		err = state.Save(t.ID, prdName, r)
	}
	unlock()
	if err != nil {
		return err
	}
	for step := r.NextStep(); step != ""; step = r.NextStep() {
		if err := runStep(ctx, t, prdName, r, step); err != nil {
			return err
		}
	}
	r.Phase, r.PID = project.PhaseMerged, 0
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	// The PRD is merged already, a log that does not open only loses the log line of the notification.
	log, err := openLog(state.LogPath(t.ID, prdName, state.StepMerge))
	if err != nil {
		log = &logger{}
	}
	defer log.close()
	notifyRun(t, r, notify.Merged, log)
	return nil
}

// prepare loads the existing run or creates one in phase waiting, so the port is fixed before the
// runner starts.
func prepare(t Target, prdName string) (*state.Run, error) {
	cfg, err := t.Repo()
	if err != nil {
		return nil, err
	}
	if r, err := state.Load(t.ID, prdName); err == nil {
		if r.Running() && r.PID != os.Getpid() {
			return nil, fmt.Errorf("%s runs already (PID %d, phase %s)", prdName, r.PID, r.Phase)
		}
		if r.NextStep() == "" {
			return nil, fmt.Errorf("nothing left to do for %s, phase %s", prdName, r.Phase)
		}
		return r, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	prds, err := project.ReadAll(t.Dir, cfg.Base(), nil)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(prds, func(d project.PRD) bool { return d.Name == prdName })
	if i < 0 {
		return nil, fmt.Errorf("no PRD %q under %s/", prdName, project.Dir)
	}
	switch d := prds[i]; d.Status.Kind {
	case project.StatusImplemented:
		return nil, fmt.Errorf("%s is merged already", d.Path)
	case project.StatusDiscarded:
		return nil, fmt.Errorf("%s is discarded", d.Path)
	}
	port, err := allocatePort()
	if err != nil {
		return nil, err
	}
	r := &state.Run{
		ProjectDir: t.Dir,
		PRD:        prds[i].Path,
		Branch:     project.FeaturePrefix + prdName,
		Base:       cfg.Base(),
		Worktree:   WorktreePath(t.ID, prdName),
		Port:       port,
		Slug:       Slug(prdName),
		Phase:      project.PhaseWaiting,
		StartedAt:  state.Now(),
	}
	// Save early, so a run starting at the same time does not take the same port.
	return r, state.Save(t.ID, prdName, r)
}

// runStep runs a step and writes its end into the state. After an error or abort the worktree stays.
func runStep(ctx context.Context, t Target, prdName string, r *state.Run, step string) error {
	r.Phase = stepPhases[step]
	r.StartStep(step)
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	log, err := openLog(state.LogPath(t.ID, prdName, step))
	if err != nil {
		return err
	}
	defer log.close()
	log.printf("Step %s", step)

	exitCode := 0
	switch step {
	case state.StepSetup:
		if err = setup(t, r, log); err != nil {
			exitCode = 1
		}
	case state.StepMerge:
		if err = merge(ctx, t, prdName, r, log); err != nil {
			exitCode = 1
		}
	default:
		exitCode, err = claudeStep(ctx, t, prdName, r, step, log)
	}

	switch {
	case ctx.Err() != nil:
		log.printf("Aborted")
		r.EndStep(exitCode, false)
		r.Phase, r.PID = project.PhaseAborted, 0
		saveErr := state.Save(t.ID, prdName, r)
		notifyRun(t, r, notify.Aborted, log)
		return errors.Join(fmt.Errorf("%s aborted", step), saveErr)
	case err != nil:
		log.printf("Error: %v", err)
		r.EndStep(exitCode, false)
		r.Fail(err)
		r.PID = 0
		saveErr := state.Save(t.ID, prdName, r)
		notifyRun(t, r, notify.Failed, log)
		return errors.Join(err, saveErr)
	}
	r.EndStep(0, true)
	switch step {
	case state.StepSetup:
		log.printf("Setup done: %s on port %d", r.Worktree, r.Port)
	case state.StepMerge:
		log.printf("Merged")
		if r.Note != "" {
			log.printf("Note: %s", r.Note)
		}
	default:
		log.printf("%s done", step)
	}
	return state.Save(t.ID, prdName, r)
}
