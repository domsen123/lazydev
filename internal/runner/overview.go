package runner

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/devserver"
	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// Item is a PRD of a project with its run, as "lazydev status" shows it and "lazydev wait"
// compares it.
type Item struct {
	Project string `json:"project"`
	PRD     string `json:"prd"`
	Label   string `json:"label"`
	// Path is relative to the project, File the absolute path where the PRD is now: in the
	// worktree of its run or in the main folder.
	Path   string        `json:"path,omitempty"`
	File   string        `json:"file,omitempty"`
	Status string        `json:"status,omitempty"`
	Phase  project.Phase `json:"phase"`
	// Queue is the 1-based position in the waiting queue of all projects, 0 if it does not wait.
	Queue int `json:"queue,omitempty"`
	// Step is the running step, or the step a start continues with.
	Step     string  `json:"step,omitempty"`
	Branch   string  `json:"branch,omitempty"`
	Worktree string  `json:"worktree,omitempty"`
	Port     int     `json:"port,omitempty"`
	Cost     float64 `json:"cost,omitempty"`
	Error    string  `json:"error,omitempty"`
	Note     string  `json:"note,omitempty"`
	// Blocker holds the open questions of a blocked run.
	Blocker *Blocker `json:"blocker,omitempty"`
	DevPID  int      `json:"devPid,omitempty"`
	Log     string   `json:"log,omitempty"`
}

// Blocker is the section "## Blocker" of a blocked PRD.
type Blocker struct {
	Step      string   `json:"step"`
	Questions []string `json:"questions"`
}

// Overview lists the PRDs of project t with their runs, sorted by label. Runs whose PRD is gone
// from prd/ come too.
func Overview(t Target) ([]Item, error) {
	entries := state.All()
	queue := waitingQueue(entries)
	runs := map[string]*state.Run{}
	var moved []project.Moved
	for _, e := range entries {
		if e.ID == t.ID {
			runs[e.PRDName] = e.Run
			moved = append(moved, project.Moved{Name: e.PRDName, Worktree: e.Run.Worktree, Path: e.Run.PRD, Branch: e.Run.Branch})
		}
	}
	prds, err := project.ReadAll(t.Dir, repo.Read(t.Dir, t.ID).Config.Base(), moved)
	if err != nil {
		return nil, err
	}
	var items []Item
	seen := map[string]bool{}
	for _, d := range prds {
		seen[d.Name] = true
		it := Item{Project: t.ID, PRD: d.Name, Label: d.Label(), Path: d.Path, File: filepath.Join(d.Root, d.Path),
			Status: d.Status.Value, Phase: d.Phase(), Cost: d.Cost}
		if r := runs[d.Name]; r != nil {
			fillRun(&it, t.ID, r, queue)
		}
		items = append(items, it)
	}
	for name, r := range runs {
		if seen[name] {
			continue
		}
		it := Item{Project: t.ID, PRD: name, Label: name, Path: r.PRD}
		fillRun(&it, t.ID, r, queue)
		items = append(items, it)
	}
	slices.SortFunc(items, func(a, b Item) int { return strings.Compare(a.Label, b.Label) })
	return items, nil
}

// fillRun adds the run of the PRD to it.
func fillRun(it *Item, id string, r *state.Run, queue []string) {
	it.Phase = r.Effective()
	it.Queue = slices.Index(queue, id+"/"+it.PRD) + 1
	it.Branch, it.Worktree, it.Port, it.Note = r.Branch, r.Worktree, r.Port, r.Note
	if r.Error != nil {
		it.Error = *r.Error
	}
	if c := r.Cost(); c > 0 {
		it.Cost = c
	}
	it.Step = r.NextStep()
	if last := r.LastStep(); last != nil && r.Running() && last.ExitCode == nil {
		it.Step = last.Name
	}
	if it.Step != "" {
		it.Log = state.LogPath(id, it.PRD, it.Step)
	}
	it.DevPID = devserver.PID(id, it.PRD)
	if r.Worktree != "" && r.PRD != "" {
		file := filepath.Join(r.Worktree, r.PRD)
		if it.File == "" {
			it.File = file
		}
		if it.Phase == project.PhaseBlocked {
			if data, err := os.ReadFile(file); err == nil {
				if step, questions, ok := project.Blocker(string(data)); ok {
					it.Blocker = &Blocker{Step: step, Questions: questions}
				}
			}
		}
	}
}

// waitingQueue lists the waiting PRDs of all projects as "<id>/<prd>" in the order they start.
func waitingQueue(entries []state.Entry) []string {
	var queue []string
	for _, e := range entries {
		if e.Run.Phase == project.PhaseWaiting && !e.Run.Running() {
			queue = append(queue, e.ID+"/"+e.PRDName)
		}
	}
	return queue
}

// Event is a change "lazydev wait" reports.
type Event struct {
	// Kind is the new phase (blocked, error, aborted, merged), "note" for a new note of a run,
	// "idle" when nothing runs or waits, or "timeout".
	Kind    string        `json:"kind"`
	Project string        `json:"project,omitempty"`
	PRD     string        `json:"prd,omitempty"`
	Phase   project.Phase `json:"phase,omitempty"`
	From    project.Phase `json:"from,omitempty"`
	Text    string        `json:"text,omitempty"`
	Item    *Item         `json:"item,omitempty"`
}

// eventPhases are the phases that end or halt a run, so an operator has to look.
var eventPhases = []project.Phase{project.PhaseBlocked, project.PhaseError, project.PhaseAborted, project.PhaseMerged}

// Changes compares two overviews and returns the events: a run that reached a phase of
// eventPhases, and a run with a new note.
func Changes(before, after []Item) []Event {
	old := map[string]Item{}
	for _, it := range before {
		old[it.Project+"/"+it.PRD] = it
	}
	var events []Event
	for _, it := range after {
		prev, known := old[it.Project+"/"+it.PRD]
		item := it
		switch {
		case (!known || prev.Phase != it.Phase) && slices.Contains(eventPhases, it.Phase) && (known || it.Phase != project.PhaseMerged):
			text := it.Error
			if it.Blocker != nil {
				text = strings.Join(it.Blocker.Questions, " | ")
			}
			events = append(events, Event{Kind: string(it.Phase), Project: it.Project, PRD: it.PRD, Phase: it.Phase, From: prev.Phase, Text: text, Item: &item})
		case it.Note != "" && it.Note != prev.Note:
			events = append(events, Event{Kind: "note", Project: it.Project, PRD: it.PRD, Phase: it.Phase, Text: it.Note, Item: &item})
		}
	}
	return events
}

// Busy reports whether a run of items works or waits.
func Busy(items []Item) bool {
	return slices.ContainsFunc(items, func(it Item) bool {
		return it.Phase == project.PhaseWaiting || it.Phase == project.PhaseMergeQueue || it.Phase.Active()
	})
}
