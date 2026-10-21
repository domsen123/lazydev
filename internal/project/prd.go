// Package project reads the PRDs of a project: prd/<area>/<name>.md with the title in the first
// heading and a status line "> Status: <value> · <detail>".
package project

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"lazydev/internal/git"
)

// Dir is the folder of the PRDs in a project.
const Dir = "prd"

// FeaturePrefix and the PRD name make the branch of a run, "feat/<name>".
const FeaturePrefix = "feat/"

// PRD is one PRD of a project.
type PRD struct {
	// Path is relative to the project, e.g. "prd/auth/token.md".
	Path string
	// Root is the folder the PRD was read from: the main folder, or the worktree of its run if
	// the run moved it there.
	Root string
	// Name is the file name without ".md", Area the folder below prd/, e.g. "auth".
	Name, Area string
	Title      string
	Status     Status
	// Tracked is false while the PRD is untracked in the main folder.
	Tracked bool
	// Branch is "feat/<name>" if it exists, the base branch for merged PRDs, empty otherwise.
	Branch string
	// Created is the earlier of the file's birth time and its first commit.
	Created time.Time
	// Cost and Model stand in the status line after a merge. HasCost is false without "Kosten:".
	Cost    float64
	HasCost bool
	Model   string
}

// Label is "<area>/<name>", like the PRD list shows it.
func (d PRD) Label() string {
	if d.Area == "" {
		return d.Name
	}
	return d.Area + "/" + d.Name
}

// Phase is the phase from the status line, for PRDs without a run.
func (d PRD) Phase() Phase {
	switch d.Status.Kind {
	case StatusPlanned:
		return PhasePlanned
	case StatusImplemented:
		return PhaseMerged
	case StatusDiscarded:
		return PhaseDiscarded
	}
	return PhaseUnknown
}

// Moved is a PRD that a run moved into its worktree, so the main folder lacks it.
type Moved struct {
	Name, Worktree, Path, Branch string
}

// ReadAll scans prd/ of the project at dir, sorted by path. Git fills in Tracked, Branch and the
// first commit, folders without git get the PRDs without that. PRDs in moved come from their
// worktree if the main folder lacks them.
func ReadAll(dir, base string, moved []Moved) ([]PRD, error) {
	var prds []PRD
	err := filepath.WalkDir(filepath.Join(dir, Dir), func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		d, err := Read(dir, path)
		if err != nil {
			return err
		}
		prds = append(prds, d)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	for _, m := range moved {
		if slices.ContainsFunc(prds, func(d PRD) bool { return d.Name == m.Name }) {
			continue
		}
		if d, err := Read(m.Worktree, filepath.Join(m.Worktree, m.Path)); err == nil {
			d.Tracked, d.Branch = true, m.Branch
			prds = append(prds, d)
		}
	}
	slices.SortFunc(prds, func(a, b PRD) int { return strings.Compare(a.Path, b.Path) })
	untracked := git.Output(dir, "ls-files", "--others", "--exclude-standard", "--", Dir)
	features := git.Output(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+FeaturePrefix)
	added := addedTimes(dir)
	for i := range prds {
		d := &prds[i]
		if d.Root != dir {
			continue
		}
		d.Tracked = !slices.Contains(untracked, d.Path)
		if at, ok := added[d.Path]; ok && (d.Created.IsZero() || at.Before(d.Created)) {
			d.Created = at
		}
		switch {
		case slices.Contains(features, FeaturePrefix+d.Name):
			d.Branch = FeaturePrefix + d.Name
		case d.Status.Kind == StatusImplemented:
			d.Branch = base
		}
	}
	return prds, err
}

// Read reads title and status line from the head of the PRD at path below root.
func Read(root, path string) (PRD, error) {
	f, err := os.Open(path)
	if err != nil {
		return PRD{}, err
	}
	defer f.Close()
	rel, _ := filepath.Rel(root, path)
	rel = filepath.ToSlash(rel)
	d := PRD{Path: rel, Root: root, Name: strings.TrimSuffix(filepath.Base(path), ".md"), Created: fileCreated(path)}
	if area := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(rel, Dir+"/"))); area != "." {
		d.Area = area
	}
	sc := bufio.NewScanner(f)
	found := false
	for i := 0; i < HeaderScanLines && sc.Scan(); i++ {
		line := sc.Text()
		if found {
			// Following lines of the same quote block belong to the detail.
			cont, ok := strings.CutPrefix(line, ">")
			if !ok {
				break
			}
			d.Status.Detail = strings.TrimSpace(d.Status.Detail + "\n" + strings.TrimPrefix(cont, " "))
			continue
		}
		if d.Title == "" {
			if t, ok := strings.CutPrefix(line, "# "); ok {
				d.Title = strings.TrimSpace(strings.TrimPrefix(t, "PRD:"))
				continue
			}
		}
		d.Status, found = ParseStatusLine(line)
	}
	d.Cost, d.HasCost, d.Model = RunInfo(d.Status)
	return d, sc.Err()
}

// fileCreated is the birth time of the file, its modification time without one.
func fileCreated(path string) time.Time {
	var stx unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_BTIME, &stx); err == nil && stx.Mask&unix.STATX_BTIME != 0 {
		return time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec))
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// addedTimes maps each path under prd/ to the time of the first commit that added it.
func addedTimes(dir string) map[string]time.Time {
	const marker = "lazydev-commit "
	added := map[string]time.Time{}
	var at time.Time
	// git log lists the newest commits first, so the last hit is the oldest.
	for _, l := range git.Output(dir, "log", "--diff-filter=A", "--relative", "--name-only", "--format="+marker+"%ct", "--", Dir) {
		if ts, ok := strings.CutPrefix(l, marker); ok {
			sec, _ := strconv.ParseInt(ts, 10, 64)
			at = time.Unix(sec, 0)
			continue
		}
		added[l] = at
	}
	return added
}
