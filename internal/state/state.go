// Package state reads and writes the state of runs under $XDG_STATE_HOME/lazydev/: one JSON file
// per PRD, the logs of its steps, locks and trust.
package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"lazydev/internal/fsutil"
	"lazydev/internal/project"
)

// The steps of a run in order.
const (
	StepSetup     = "setup"
	StepImplement = "implement"
	StepReview    = "review"
	StepMerge     = "merge"
)

var Steps = []string{StepSetup, StepImplement, StepReview, StepMerge}

// BinaryName is the name runners run under. IsRunner recognizes them by it.
const BinaryName = "lazydev"

// Dir honors XDG_STATE_HOME.
func Dir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "lazydev")
}

func RunsDir(id string) string             { return filepath.Join(Dir(), "runs", id) }
func Path(id, prdName string) string       { return filepath.Join(RunsDir(id), prdName+".json") }
func RunnerLogPath(id, name string) string { return filepath.Join(RunsDir(id), name+".runner.log") }

// LogPath is the log of one step, e.g. "setup" or "implement".
func LogPath(id, prdName, step string) string {
	return filepath.Join(RunsDir(id), prdName+"."+step+".jsonl")
}

// Step is one started step of a run.
type Step struct {
	Name      string `json:"name"`
	SessionID string `json:"sessionId,omitempty"`
	// Model comes from the first init event of claude in the step.
	Model    string `json:"model,omitempty"`
	ExitCode *int   `json:"exitCode"`
	// OK is false if the step failed despite exit code 0, e.g. on an open question.
	OK        bool     `json:"ok"`
	CostUSD   *float64 `json:"costUsd,omitempty"`
	StartedAt string   `json:"startedAt,omitempty"`
	EndedAt   string   `json:"endedAt,omitempty"`
}

// Run is the state of a PRD between start and cleanup.
type Run struct {
	// ProjectDir is the main folder, so a runner can start waiting PRDs of any project.
	ProjectDir string        `json:"projectDir"`
	PRD        string        `json:"prd"`
	Branch     string        `json:"branch"`
	Base       string        `json:"base"`
	Worktree   string        `json:"worktree"`
	Port       int           `json:"port"`
	Slug       string        `json:"slug"`
	Phase      project.Phase `json:"phase"`
	PID        int           `json:"pid"`
	StartedAt  string        `json:"startedAt"`
	Steps      []Step        `json:"steps"`
	Error      *string       `json:"error"`
	// Note is a hint for humans, e.g. that the main folder blocks the merge.
	Note string `json:"note,omitempty"`
}

// Load returns fs.ErrNotExist without a run.
func Load(id, prdName string) (*Run, error) {
	data, err := os.ReadFile(Path(id, prdName))
	if err != nil {
		return nil, err
	}
	r := &Run{}
	return r, json.Unmarshal(data, r)
}

// Save writes atomically, so the TUI never reads half a file.
func Save(id, prdName string, r *Run) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(Path(id, prdName), append(data, '\n'), 0o644)
}

// Remove deletes state and logs of a PRD.
func Remove(id, prdName string) error {
	files := []string{Path(id, prdName), RunnerLogPath(id, prdName), DevPIDPath(id, prdName), DevLogPath(id, prdName)}
	for _, step := range Steps {
		files = append(files, LogPath(id, prdName, step))
	}
	var errs []error
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Entry is a stored run with project ID and PRD name from its path.
type Entry struct {
	ID, PRDName string
	Run         *Run
}

// All reads the runs of all projects, sorted by start. Unreadable files are skipped.
func All() []Entry {
	files, _ := filepath.Glob(filepath.Join(Dir(), "runs", "*", "*.json"))
	var entries []Entry
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		r := &Run{}
		if json.Unmarshal(data, r) != nil {
			continue
		}
		entries = append(entries, Entry{ID: filepath.Base(filepath.Dir(f)), PRDName: strings.TrimSuffix(filepath.Base(f), ".json"), Run: r})
	}
	slices.SortStableFunc(entries, func(a, b Entry) int { return strings.Compare(a.Run.StartedAt, b.Run.StartedAt) })
	return entries
}

// ProjectRuns maps the PRD names of project id to their runs.
func ProjectRuns(id string) map[string]*Run {
	runs := map[string]*Run{}
	for _, e := range All() {
		if e.ID == id {
			runs[e.PRDName] = e.Run
		}
	}
	return runs
}

// Now is a timestamp in the format of the state.
func Now() string { return time.Now().Format(time.RFC3339) }

func (r *Run) StartStep(name string) {
	r.Steps = append(r.Steps, Step{Name: name, StartedAt: Now()})
}

// EndStep finishes the last started step.
func (r *Run) EndStep(exitCode int, ok bool) {
	if s := r.LastStep(); s != nil {
		s.ExitCode, s.OK, s.EndedAt = &exitCode, ok, Now()
	}
}

// LastStep is nil while no step started.
func (r *Run) LastStep() *Step {
	if len(r.Steps) == 0 {
		return nil
	}
	return &r.Steps[len(r.Steps)-1]
}

func (r *Run) Fail(err error) {
	msg := err.Error()
	r.Error, r.Phase = &msg, project.PhaseError
}

// Cost adds up the cost of all steps in USD.
func (r *Run) Cost() float64 {
	total := 0.0
	for _, s := range r.Steps {
		if s.CostUSD != nil {
			total += *s.CostUSD
		}
	}
	return total
}

// ImplementModel is the model of the last successful implement step, empty otherwise.
func (r *Run) ImplementModel() string {
	for i := len(r.Steps) - 1; i >= 0; i-- {
		if s := r.Steps[i]; s.Name == StepImplement && s.OK {
			return s.Model
		}
	}
	return ""
}

// Running reports whether the runner of the run lives.
func (r *Run) Running() bool {
	return r.PID > 0 && IsRunner(r.PID)
}

// Effective is the phase to show: an active phase without a living runner counts as aborted.
func (r *Run) Effective() project.Phase {
	if r.Phase.Active() && !r.Running() {
		return project.PhaseAborted
	}
	return r.Phase
}

// NextStep is the step a run continues with: the first, the failed one after an error or abort,
// otherwise the next. After the merge it is empty.
func (r *Run) NextStep() string {
	last := r.LastStep()
	if last == nil {
		return Steps[0]
	}
	if last.ExitCode == nil || !last.OK {
		return last.Name
	}
	i := slices.Index(Steps, last.Name)
	if i < 0 || i+1 >= len(Steps) {
		return ""
	}
	return Steps[i+1]
}

// IsRunner reports whether pid lives and runs "lazydev run". A reused PID of another process does
// not count.
func IsRunner(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return false
	}
	args := strings.Split(string(bytes.TrimRight(data, "\x00")), "\x00")
	return len(args) >= 2 && filepath.Base(args[0]) == BinaryName && args[1] == "run"
}
