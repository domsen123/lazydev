package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/devserver"
	"lazydev/internal/fsutil"
	"lazydev/internal/git"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
	"lazydev/internal/repo"
	"lazydev/internal/runner"
	"lazydev/internal/skill"
	"lazydev/internal/state"
)

// The commands below make the TUI optional: everything the TUI does to projects, PRDs and runs
// works from the command line too. A command that takes "[<project>]" uses the project of the
// working directory without it.

// cwdTarget is the project of the working directory: the main folder of its repository, also from
// a subfolder or the worktree of a run.
func cwdTarget() (runner.Target, error) {
	dir, err := os.Getwd()
	if err != nil {
		return runner.Target{}, err
	}
	return runner.FindDir(dir)
}

// targetArgs splits args into the project and the remaining n arguments. With n+1 arguments the
// first one names the project, with n the working directory does.
func targetArgs(args []string, n int) (runner.Target, []string, error) {
	switch len(args) {
	case n + 1:
		t, err := runner.Find(args[0])
		return t, args[1:], err
	case n:
		t, err := cwdTarget()
		return t, args, err
	}
	usageError()
	return runner.Target{}, nil, nil
}

// flags removes the given boolean flags from args and reports which were set.
func flags(args []string, names ...string) ([]string, map[string]bool) {
	set := map[string]bool{}
	var rest []string
	for _, a := range args {
		if slices.Contains(names, a) {
			set[a] = true
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n\n", a)
			usageError()
		}
		rest = append(rest, a)
	}
	return rest, set
}

// valueFlag removes "--name <value>" from args.
func valueFlag(args []string, name string) ([]string, string, bool) {
	i := slices.Index(args, name)
	if i < 0 {
		return args, "", false
	}
	if i+1 >= len(args) {
		usageError()
	}
	return slices.Delete(slices.Clone(args), i, i+2), args[i+1], true
}

// addProject is "lazydev add [<path>]": it adds the folder, without path the repository of the
// working directory, and runs the init of .lazydev/. A folder in the list only gets the init.
func addProject(args []string) error {
	if len(args) > 1 {
		usageError()
	}
	input := "."
	if len(args) == 1 {
		input = args[0]
	}
	path, err := config.ExpandPath(input)
	if err != nil {
		return err
	}
	if !fsutil.IsDir(path) {
		return fmt.Errorf("not a folder: %s", config.DisplayPath(path))
	}
	if len(args) == 0 {
		path = git.MainDir(path)
	}
	if err := repo.Init(path); err != nil {
		return err
	}
	added, err := config.AddProject(path)
	if err != nil {
		return err
	}
	t, err := runner.Find(path)
	if err != nil {
		return err
	}
	if added {
		fmt.Printf("Added %s (%s).\n", t.ID, config.DisplayPath(path))
	} else {
		fmt.Printf("%s (%s) is in the list already, %s/ is complete.\n", t.ID, config.DisplayPath(path), repo.Dir)
	}
	// Like the start of the TUI, adding keeps the skill current.
	if config.LoadSettings().SkillEnabled() {
		if _, err := skill.Install(); err != nil {
			fmt.Fprintln(os.Stderr, "lazydev: skill not installed:", err)
		}
	}
	return nil
}

// removeProject is "lazydev remove <project>": it drops the project from the list and keeps folder,
// runs and worktrees.
func removeProject(args []string) error {
	if len(args) != 1 {
		usageError()
	}
	t, err := runner.Find(args[0])
	if err != nil {
		return err
	}
	if err := config.RemoveProject(t.Dir); err != nil {
		return err
	}
	fmt.Printf("Removed %s from the list, the folder stays.\n", t.ID)
	if items, err := runner.Overview(t); err == nil && runner.Busy(items) {
		fmt.Println("Its runners keep working until they end.")
	}
	return nil
}

// listProjects is "lazydev projects [--json]".
func listProjects(args []string) error {
	args, set := flags(args, "--json")
	if len(args) != 0 {
		usageError()
	}
	paths, err := config.LoadProjects()
	if err != nil {
		return err
	}
	type entry struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	entries := []entry{}
	for i, id := range config.IDs(paths) {
		entries = append(entries, entry{ID: id, Path: paths[i]})
	}
	if set["--json"] {
		return writeJSON(entries)
	}
	if len(entries) == 0 {
		fmt.Println("No projects. Add one with lazydev add [<path>].")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\n", e.ID, config.DisplayPath(e.Path))
	}
	return w.Flush()
}

// scopeTargets are the projects of status and wait: the named one, without a name the project of
// the working directory, outside a project all projects.
func scopeTargets(args []string) ([]runner.Target, error) {
	switch len(args) {
	case 1:
		t, err := runner.Find(args[0])
		return []runner.Target{t}, err
	case 0:
		if t, err := cwdTarget(); err == nil {
			return []runner.Target{t}, nil
		}
		paths, err := config.LoadProjects()
		if err != nil {
			return nil, err
		}
		var targets []runner.Target
		for i, id := range config.IDs(paths) {
			targets = append(targets, runner.Target{ID: id, Dir: paths[i]})
		}
		return targets, nil
	}
	usageError()
	return nil, nil
}

func overview(targets []runner.Target) ([]runner.Item, error) {
	var items []runner.Item
	var errs []error
	for _, t := range targets {
		its, err := runner.Overview(t)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.ID, err))
		}
		items = append(items, its...)
	}
	return items, errors.Join(errs...)
}

// status is "lazydev status [<project>] [--all] [--json]": the open PRDs with phase, step, port and
// cost, blocked runs with their questions. --all adds merged and discarded PRDs.
func status(args []string) error {
	args, set := flags(args, "--json", "--all")
	targets, err := scopeTargets(args)
	if err != nil {
		return err
	}
	items, err := overview(targets)
	if err != nil && items == nil {
		return err
	}
	if !set["--all"] {
		items = slices.DeleteFunc(items, func(it runner.Item) bool { return it.Phase.Done() && it.Error == "" })
	}
	if set["--json"] {
		if items == nil {
			items = []runner.Item{}
		}
		if werr := writeJSON(items); werr != nil {
			return werr
		}
		return err
	}
	printStatus(os.Stdout, items, len(targets) > 1)
	return err
}

func printStatus(out io.Writer, items []runner.Item, withProject bool) {
	if len(items) == 0 {
		fmt.Fprintln(out, "No open PRDs.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	head := "PRD\tPHASE\tSTEP\tPORT\tCOST\tSTATUS"
	if withProject {
		head = "PROJECT\t" + head
	}
	fmt.Fprintln(w, head)
	for _, it := range items {
		phase := string(it.Phase)
		if it.Queue > 0 {
			phase += fmt.Sprintf(" #%d", it.Queue)
		}
		if len(it.WaitsFor) > 0 && it.Phase == project.PhaseWaiting {
			phase += " → " + strings.Join(it.WaitsFor, ", ")
		}
		port, cost := "", ""
		if it.Port > 0 {
			port = fmt.Sprint(it.Port)
			if it.DevPID > 0 {
				port += " dev"
			}
		}
		if it.Cost > 0 {
			cost = fmt.Sprintf("%.2f", it.Cost)
		}
		row := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", it.Label, phase, it.Step, port, cost, it.Status)
		if withProject {
			row = it.Project + "\t" + row
		}
		fmt.Fprintln(w, row)
	}
	w.Flush()
	for _, it := range items {
		name := it.Label
		if withProject {
			name = it.Project + "/" + name
		}
		if it.Error != "" {
			fmt.Fprintf(out, "\n%s: %s\n", name, it.Error)
		}
		if it.Note != "" {
			fmt.Fprintf(out, "\n%s: %s\n", name, it.Note)
		}
		if len(it.WaitsFor) > 0 && it.Phase == project.PhaseWaiting {
			fmt.Fprintf(out, "\n%s waits for %s: %s\n", name, strings.Join(it.WaitsFor, ", "), it.AfterReason)
		}
		if it.Blocker != nil {
			fmt.Fprintf(out, "\n%s is blocked in %s:\n", name, it.Blocker.Step)
			for _, q := range it.Blocker.Questions {
				fmt.Fprintf(out, "  - %s\n", q)
			}
		}
	}
}

// wait is "lazydev wait [<project>] [--json] [--timeout <duration>]": it blocks until a run gets
// blocked, fails, aborts or merges, or gets a note, and prints the events. With nothing running or
// waiting it returns right away with "idle".
func wait(args []string) error {
	args, timeoutText, hasTimeout := valueFlag(args, "--timeout")
	args, set := flags(args, "--json")
	var deadline time.Time
	if hasTimeout {
		d, err := time.ParseDuration(timeoutText)
		if err != nil || d <= 0 {
			return fmt.Errorf("--timeout %q is no duration like 30m", timeoutText)
		}
		deadline = time.Now().Add(d)
	}
	targets, err := scopeTargets(args)
	if err != nil {
		return err
	}
	before, err := overview(targets)
	if err != nil && before == nil {
		return err
	}
	for {
		var events []runner.Event
		if !runner.Busy(before) {
			events = []runner.Event{{Kind: "idle", Text: "no run works or waits"}}
		} else if !deadline.IsZero() && time.Now().After(deadline) {
			events = []runner.Event{{Kind: "timeout", Text: "no event within " + timeoutText}}
		} else {
			time.Sleep(time.Second)
			after, err := overview(targets)
			if err != nil && after == nil {
				return err
			}
			events = runner.Changes(before, after)
			before = after
		}
		if len(events) == 0 {
			continue
		}
		if set["--json"] {
			enc := json.NewEncoder(os.Stdout)
			for _, e := range events {
				if err := enc.Encode(e); err != nil {
					return err
				}
			}
			return nil
		}
		for _, e := range events {
			name := e.PRD
			if e.Project != "" {
				name = e.Project + "/" + e.PRD
			}
			line := strings.TrimSpace(e.Kind + " " + name)
			if e.Text != "" {
				line += ": " + e.Text
			}
			fmt.Println(line)
		}
		return nil
	}
}

// showLog is "lazydev log [<project>] <prd> [-f]": the step logs of the run in step order, like
// the log tab of the TUI. -f follows them until the runner ends.
func showLog(args []string) error {
	args, set := flags(args, "-f", "--follow")
	follow := set["-f"] || set["--follow"]
	t, rest, err := targetArgs(args, 1)
	if err != nil {
		return err
	}
	name := rest[0]
	if _, err := state.Load(t.ID, name); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s has no run", name)
	}
	offsets := map[string]int64{}
	for {
		for _, step := range state.Steps {
			path := state.LogPath(t.ID, name, step)
			lines, next := readNewLines(path, offsets[step])
			offsets[step] = next
			for _, line := range lines {
				for _, e := range claude.Describe(line) {
					printLogEntry(step, e)
				}
			}
		}
		if !follow {
			return nil
		}
		r, err := state.Load(t.ID, name)
		if err != nil {
			return err
		}
		if !r.Running() && r.Phase != project.PhaseWaiting {
			fmt.Printf("-- %s %s\n", name, r.Effective())
			return nil
		}
		time.Sleep(time.Second)
	}
}

// readNewLines reads the complete lines of path from offset on. The last line may still be
// written, it waits for the next read.
func readNewLines(path string, offset int64) ([][]byte, int64) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, offset
	}
	if int64(len(data)) < offset {
		offset = 0
	}
	data = data[offset:]
	end := strings.LastIndexByte(string(data), '\n')
	if end < 0 {
		return nil, offset
	}
	var lines [][]byte
	for l := range strings.SplitSeq(string(data[:end]), "\n") {
		lines = append(lines, []byte(l))
	}
	return lines, offset + int64(end) + 1
}

func printLogEntry(step string, e claude.Entry) {
	at := "        "
	if !e.At.IsZero() {
		at = e.At.Local().Format("15:04:05")
	}
	indent := ""
	if e.Sub {
		indent = "  "
	}
	text := strings.ReplaceAll(e.Text, "\n", "\n"+strings.Repeat(" ", 8+1+len(step)+1+len(e.Kind)+1)+indent)
	fmt.Printf("%s %s %s %s%s\n", at, step, e.Kind, indent, text)
}

// setAfter is "lazydev after [<project>] <prd> [<prd>…]": it overrides the decision of the step plan.
// The project is only optional with exactly one argument; with more, the first one names a project
// when it is one, otherwise all name PRDs of the project of the working directory.
func setAfter(args []string) error {
	if len(args) == 0 {
		usageError()
	}
	var t runner.Target
	var err error
	if len(args) >= 2 {
		if found, ferr := runner.Find(args[0]); ferr == nil {
			t, args = found, args[1:]
		}
	}
	if t.ID == "" {
		if t, err = cwdTarget(); err != nil {
			return err
		}
	}
	name, deps := args[0], args[1:]
	if err := runner.SetAfter(t, name, deps); err != nil {
		return err
	}
	if len(deps) == 0 {
		fmt.Printf("%s waits for nothing and starts with the next free slot.\n", name)
	} else {
		fmt.Printf("%s starts once %s are merged.\n", name, strings.Join(deps, ", "))
	}
	return nil
}

// setStatus is "lazydev set-status [<project>] <prd> planned|merged|discarded", the status picker of
// the TUI: a tracked PRD gets committed alone.
func setStatus(args []string) error {
	t, rest, err := targetArgs(args, 2)
	if err != nil {
		return err
	}
	name, value := rest[0], rest[1]
	i := slices.IndexFunc(project.Choices, func(c project.Choice) bool { return c.Label == value })
	if i < 0 {
		return fmt.Errorf("unknown status %q, use planned, merged or discarded", value)
	}
	cfg, err := t.Repo()
	if err != nil {
		return err
	}
	prds, err := project.ReadAll(t.Dir, cfg.Base(), nil)
	if err != nil {
		return err
	}
	j := slices.IndexFunc(prds, func(d project.PRD) bool { return d.Name == name })
	if j < 0 {
		if r, err := state.Load(t.ID, name); err == nil && r.Worktree != "" {
			return fmt.Errorf("%s is in the worktree of its run, its status belongs to the run", name)
		}
		return fmt.Errorf("no PRD %s in %s", name, t.ID)
	}
	c := project.Choices[i]
	committed, err := project.ChangeStatus(t.Dir, cfg.Base(), prds[j].Path, c, time.Now())
	if err != nil {
		return err
	}
	text := fmt.Sprintf("Status of %s: %s", prds[j].Label(), c.Label)
	if committed {
		text += ", committed"
	}
	fmt.Println(text + ".")
	return nil
}

// dev is "lazydev dev [<project>] <prd> [start|stop]": without action it toggles like D in the TUI.
func dev(args []string) error {
	action := ""
	if n := len(args); n > 1 && (args[n-1] == "start" || args[n-1] == "stop") {
		action, args = args[n-1], args[:n-1]
	}
	t, rest, err := targetArgs(args, 1)
	if err != nil {
		return err
	}
	name := rest[0]
	if _, err := state.Load(t.ID, name); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s has no worktree yet, start it with lazydev start", name)
	}
	running := devserver.PID(t.ID, name) != 0
	if action == "" {
		action = "start"
		if running {
			action = "stop"
		}
	}
	if action == "stop" {
		if !running {
			fmt.Printf("No dev server of %s runs.\n", name)
			return nil
		}
		if err := devserver.Stop(t.ID, name); err != nil {
			return err
		}
		fmt.Printf("Stopped the dev server of %s.\n", name)
		return nil
	}
	pid, err := runner.StartDev(t, name)
	if err != nil {
		return err
	}
	r, _ := state.Load(t.ID, name)
	fmt.Printf("Started the dev server of %s (PID %d, port %d), log: %s\n", name, pid, r.Port, state.DevLogPath(t.ID, name))
	return nil
}

// promptUnblock is "lazydev prompt unblock [--auto] [<project>] <prd>": the unblock prompt of a
// blocked run, the same one f opens in the TUI. The PRD path in it is absolute, so a session
// outside the worktree edits the right file. --auto makes Claude answer the questions itself.
func promptUnblock(args []string) error {
	args, set := flags(args, "--auto")
	t, rest, err := targetArgs(args, 1)
	if err != nil {
		return err
	}
	name := rest[0]
	r, err := state.Load(t.ID, name)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s has no run", name)
	} else if err != nil {
		return err
	}
	if r.Effective() != project.PhaseBlocked {
		return fmt.Errorf("%s is not blocked, phase %s", name, r.Effective())
	}
	if err := repo.Init(t.Dir); err != nil {
		return err
	}
	cfg, err := t.Repo()
	if err != nil {
		return err
	}
	vars := prompts.Vars{Arguments: filepath.Join(r.Worktree, r.PRD), Base: r.Base, Branch: r.Branch}
	prompt, err := prompts.Build(t.Dir, r.Worktree, cfg, "unblock", vars)
	if err != nil {
		return err
	}
	if set["--auto"] {
		prompt += "\n" + prompts.Auto("unblock", vars)
	}
	fmt.Print(prompt)
	return nil
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
