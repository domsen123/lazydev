// Package runner führt die Schritte eines PRDs aus und schreibt Zustand und Log.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"

	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/hooks"
	"lazydev/internal/project"
	"lazydev/internal/runlog"
	"lazydev/internal/state"
	"lazydev/internal/worktree"
)

// Target ist ein eingetragenes Projekt mit seiner ID.
type Target struct {
	ID     string
	Config config.Project
}

func (t Target) Dir() string {
	return t.Config.AbsPath()
}

// Repo liefert die wirksamen Einstellungen des Projekts. Der Runner liest sie bei jedem Schritt
// neu aus dem Hauptordner, uncommittete Änderungen an .lazydev/config.yaml gelten also sofort.
func (t Target) Repo() (config.Repo, error) {
	r, _, err := config.Effective(t.Config)
	return r, err
}

// Hooks liefert die Hooks aus den wirksamen Einstellungen.
func (t Target) Hooks() (hooks.Hooks, error) {
	r, err := t.Repo()
	if err != nil {
		return hooks.Hooks{}, err
	}
	return hooks.For(t.Dir(), r), nil
}

// CheckTrust prüft, dass Hooks und Freigaben aus .lazydev/config.yaml bestätigt sind. Ohne die
// Datei stammen sie aus der eigenen zentralen Konfiguration und brauchen keine Bestätigung.
func CheckTrust(t Target) error {
	r, exists, err := config.LoadRepo(t.Dir())
	if err != nil || !exists {
		return err
	}
	if !state.Trusted(t.ID, r.TrustHash()) {
		return fmt.Errorf("Hooks oder Freigaben in %s/config.yaml nicht bestätigt. Prüfen und bestätigen: lazydev trust %s",
			config.RepoDir, t.ID)
	}
	return nil
}

// FindProject sucht ein eingetragenes Projekt über seine ID.
func FindProject(cfg *config.Config, id string) (Target, error) {
	paths := make([]string, len(cfg.Projects))
	for i, p := range cfg.Projects {
		paths[i] = p.AbsPath()
	}
	for i, pid := range project.AssignIDs(paths) {
		if pid == id {
			return Target{ID: id, Config: cfg.Projects[i]}, nil
		}
	}
	return Target{}, fmt.Errorf("kein Projekt mit der ID %q eingetragen", id)
}

const (
	StepSetup     = "setup"
	StepImplement = "implement"
	StepReview    = "review"
	StepMerge     = "merge"
)

// steps ist die Reihenfolge der Schritte eines PRDs.
var steps = []string{StepSetup, StepImplement, StepReview, StepMerge}

var stepPhases = map[string]project.Phase{
	StepSetup:     project.PhaseSetup,
	StepImplement: project.PhaseImplement,
	StepReview:    project.PhaseReview,
	// Der Merge beginnt in der Merge-Queue. Mit dem Merge-Slot wechselt die Phase auf "Merge".
	StepMerge: project.PhaseMergeQueue,
}

// startLock schützt das Zählen der laufenden Runner und das Starten neuer Runner.
const startLock = "start"

// NextStep liefert den Schritt, mit dem ein Lauf weitergeht: den ersten, nach einem Fehler
// oder Abbruch den gescheiterten, sonst den folgenden. Nach dem Merge ist er leer.
func NextStep(r *state.Run) string {
	last := r.LastStep()
	if last == nil {
		return steps[0]
	}
	if last.ExitCode == nil || !last.OK {
		return last.Name
	}
	i := slices.Index(steps, last.Name)
	if i < 0 || i+1 >= len(steps) {
		return ""
	}
	return steps[i+1]
}

// Run führt die offenen Schritte eines PRDs im Vordergrund aus: Setup, Umsetzung, Review und
// Merge. Gibt es schon einen Lauf, macht Run beim gescheiterten oder nächsten Schritt weiter.
// Ohne autoMerge endet Run vor dem Merge in der Merge-Queue, bis Merge den Merge freigibt.
// Run beachtet maxParallel nicht, das übernimmt Start.
// Endet Run, startet es wartende PRDs, bis maxParallel erreicht ist.
func Run(cfg *config.Config, t Target, prdName string, echo io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	err := run(ctx, cfg, t, prdName, echo)
	if nextErr := StartWaiting(cfg); nextErr != nil {
		fmt.Fprintln(echo, "lazydev: wartende PRDs nicht gestartet:", nextErr)
	}
	return err
}

func run(ctx context.Context, cfg *config.Config, t Target, prdName string, echo io.Writer) error {
	if err := checkClaude(cfg); err != nil {
		return err
	}
	if err := CheckTrust(t); err != nil {
		return err
	}
	unlock, err := state.Lock(startLock)
	if err != nil {
		return err
	}
	r, err := prepare(cfg, t, prdName)
	if err == nil {
		r.PID = os.Getpid()
		r.Error = nil
		err = state.Save(t.ID, prdName, r)
	}
	unlock()
	if err != nil {
		return err
	}

	for step := NextStep(r); step != ""; step = NextStep(r) {
		if step == StepMerge && !t.Config.AutoMergeEnabled() && !r.MergeApproved {
			r.Phase = project.PhaseMergeQueue
			r.Note = fmt.Sprintf("autoMerge ist aus. Zum Mergen: lazydev merge %s %s", t.ID, prdName)
			r.PID = 0
			return state.Save(t.ID, prdName, r)
		}
		if err := runStep(ctx, cfg, t, prdName, r, step, echo); err != nil {
			return err
		}
	}
	r.Phase = project.PhaseMerged
	r.PID = 0
	return state.Save(t.ID, prdName, r)
}

// prepare lädt den bestehenden Lauf oder legt einen neuen in Phase "wartet" an. Der Port steht
// damit fest, bevor der Runner startet.
func prepare(cfg *config.Config, t Target, prdName string) (*state.Run, error) {
	repo, err := t.Repo()
	if err != nil {
		return nil, err
	}
	if r, err := state.Load(t.ID, prdName); err == nil {
		if isRunning(r) && r.PID != os.Getpid() {
			return nil, fmt.Errorf("für %s läuft schon ein Runner (PID %d, Phase %s)", prdName, r.PID, r.Phase.Label())
		}
		if NextStep(r) == "" {
			return nil, fmt.Errorf("für %s ist nichts mehr auszuführen, Phase %s. Zum Verwerfen: lazydev teardown %s %s",
				prdName, r.Phase.Label(), t.ID, prdName)
		}
		// Läufe ohne Base stammen aus der Zeit vor baseBranch.
		if r.Base == "" {
			r.Base = repo.Base()
		}
		return r, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	p := project.Load(t.ID, t.Dir(), repo.Base())
	if p.Err != nil {
		return nil, p.Err
	}
	prd, ok := p.FindPRD(prdName)
	if !ok {
		return nil, fmt.Errorf("kein PRD %q unter %s/", prdName, project.PRDDir)
	}
	if prd.Status.Kind == project.StatusImplemented {
		return nil, fmt.Errorf("%s ist schon umgesetzt", prd.Path)
	}
	used, err := state.UsedPorts()
	if err != nil {
		return nil, err
	}
	port, err := worktree.AllocatePort(cfg.PortBase, used)
	if err != nil {
		return nil, err
	}
	r := &state.Run{
		PRD:       prd.Path,
		Branch:    worktree.Branch(prdName),
		Base:      repo.Base(),
		Worktree:  worktree.Path(t.ID, prdName),
		Port:      port,
		Slug:      worktree.Slug(prdName),
		Phase:     project.PhaseWaiting,
		StartedAt: state.Now(),
	}
	// Früh speichern, damit ein parallel startender Lauf den Port nicht doppelt vergibt.
	return r, state.Save(t.ID, prdName, r)
}

// runStep führt einen Schritt aus und schreibt sein Ende in den Zustand. Bei Fehler oder
// Abbruch bleibt der Worktree erhalten.
func runStep(ctx context.Context, cfg *config.Config, t Target, prdName string, r *state.Run, step string, echo io.Writer) error {
	r.Phase = stepPhases[step]
	r.StartStep(step)
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	log, err := runlog.Open(state.LogPath(t.ID, prdName, step), echo)
	if err != nil {
		return err
	}
	defer log.Close()
	log.Printf("Schritt %s", r.Phase.Label())

	exitCode := 0
	switch step {
	case StepSetup:
		if err = setup(t, r, log); err != nil {
			exitCode = 1
		}
	case StepMerge:
		exitCode, err = mergeStep(ctx, cfg, t, prdName, r, log)
	default:
		exitCode, err = claudeStep(ctx, cfg, t, prdName, r, step, log)
	}

	switch {
	case ctx.Err() != nil:
		log.Printf("Abgebrochen")
		r.EndStep(exitCode, false)
		r.Phase = project.PhaseAborted
		r.PID = 0
		return errors.Join(fmt.Errorf("%s abgebrochen", stepPhases[step].Label()), state.Save(t.ID, prdName, r))
	case err != nil:
		log.Printf("Fehler: %v", err)
		r.EndStep(exitCode, false)
		r.Fail(err)
		r.PID = 0
		return errors.Join(err, state.Save(t.ID, prdName, r))
	}
	r.EndStep(0, true)
	switch step {
	case StepSetup:
		log.Printf("Setup fertig: %s auf Port %d", r.Worktree, r.Port)
	case StepMerge:
		log.Printf("Gemergt")
		if r.Note != "" {
			log.Printf("Hinweis: %s", r.Note)
		}
	default:
		log.Printf("%s fertig", stepPhases[step].Label())
	}
	return state.Save(t.ID, prdName, r)
}

// setup ist wiederholbar: Ein bestehender Worktree und ein schon verschobenes PRD bleiben
// stehen, und die Hooks überspringen vorhandene Datenbanken.
func setup(t Target, r *state.Run, log *runlog.Logger) error {
	if fileExists(r.Worktree) {
		log.Printf("Worktree %s existiert schon", r.Worktree)
	} else {
		log.Printf("Worktree %s auf Branch %s von %s", r.Worktree, r.Branch, r.Base)
		if err := worktree.Create(t.Dir(), r.Worktree, r.Branch, r.Base); err != nil {
			return err
		}
	}
	if fileExists(filepath.Join(t.Dir(), r.PRD)) && !git.IsTracked(t.Dir(), r.PRD) {
		if err := movePRD(t.Dir(), r.Worktree, r.PRD, log); err != nil {
			return err
		}
	}
	h, err := t.Hooks()
	if err != nil {
		return err
	}
	return h.Setup(env(t, r, log))
}

// movePRD verschiebt ein ungetracktes PRD aus dem Hauptordner in den Worktree und committet
// es dort. So bleibt der Hauptordner sauber, und das PRD kommt mit dem Merge in den Ziel-Branch.
func movePRD(mainDir, wt, rel string, log *runlog.Logger) error {
	if err := moveFile(filepath.Join(mainDir, rel), filepath.Join(wt, rel)); err != nil {
		return err
	}
	log.Printf("PRD %s in den Worktree verschoben", rel)
	if _, err := git.Run(wt, "add", "--", rel); err != nil {
		return err
	}
	name := filepath.Base(rel)
	msg := "docs(prd): add " + name[:len(name)-len(filepath.Ext(name))]
	if _, err := git.Run(wt, "commit", "-m", msg, "--", rel); err != nil {
		return err
	}
	log.Printf("Commit: %s", msg)
	return nil
}

func env(t Target, r *state.Run, log *runlog.Logger) hooks.Env {
	return hooks.Env{
		ProjectDir: t.Dir(),
		Worktree:   r.Worktree,
		Branch:     r.Branch,
		Port:       r.Port,
		Slug:       r.Slug,
		Log:        log,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// moveFile fällt auf Kopieren zurück, wenn Quelle und Ziel auf verschiedenen Dateisystemen liegen.
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// checkClaude prüft vor dem Start, dass das claude-Programm existiert. Sonst scheitert der Lauf
// erst nach dem Setup.
func checkClaude(cfg *config.Config) error {
	if _, err := exec.LookPath(cfg.Claude.Binary); err != nil {
		return fmt.Errorf("%s nicht gefunden. Installiere Claude Code oder setze claude.binary in der Konfiguration", cfg.Claude.Binary)
	}
	return nil
}
