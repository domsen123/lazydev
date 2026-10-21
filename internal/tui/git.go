package tui

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// gitInterval is how often the git info of all projects refreshes.
const gitInterval = 5 * time.Second

// gitInfo is the state of a project's repository for the second line in the left column.
type gitInfo struct {
	// ok is false for folders that are no git repository.
	ok bool
	// branch is the checked out branch, or the short commit for a detached HEAD.
	branch string
	// upstream is true if the branch tracks a remote branch. Only then ahead and behind count.
	upstream      bool
	ahead, behind int
}

// gitInfoMsg carries fresh git info for the project at path.
type gitInfoMsg struct {
	path string
	info gitInfo
}

// gitTickMsg triggers the periodic refresh.
type gitTickMsg struct{}

func gitTick() tea.Cmd {
	return tea.Tick(gitInterval, func(time.Time) tea.Msg { return gitTickMsg{} })
}

// fetchGitInfo reads the git info of the projects in the background, one message per project.
func fetchGitInfo(projects []*projectEntry) tea.Cmd {
	cmds := make([]tea.Cmd, len(projects))
	for i, p := range projects {
		path := p.path
		cmds[i] = func() tea.Msg { return gitInfoMsg{path: path, info: readGitInfo(path)} }
	}
	return tea.Batch(cmds...)
}

// readGitInfo runs "git status --porcelain=v2 --branch", which reports branch, upstream and the
// ahead/behind counts in its header lines:
//
//	# branch.oid 1a2b3c…
//	# branch.head main
//	# branch.upstream origin/main
//	# branch.ab +2 -1
func readGitInfo(path string) gitInfo {
	out, err := exec.Command("git", "-C", path, "status", "--porcelain=v2", "--branch", "--untracked-files=no").Output()
	if err != nil {
		return gitInfo{}
	}
	info := gitInfo{ok: true}
	var oid string
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "#" {
			continue
		}
		switch fields[1] {
		case "branch.oid":
			oid = fields[2]
		case "branch.head":
			info.branch = fields[2]
		case "branch.upstream":
			info.upstream = true
		case "branch.ab":
			info.ahead, _ = strconv.Atoi(strings.TrimPrefix(fields[2], "+"))
			if len(fields) > 3 {
				info.behind, _ = strconv.Atoi(strings.TrimPrefix(fields[3], "-"))
			}
		}
	}
	if info.branch == "(detached)" && len(oid) >= 7 {
		info.branch = oid[:7]
	}
	return info
}

// summary is the second line of a project row, e.g. "main ↑2 ↓1".
func (g gitInfo) summary() string {
	switch {
	case !g.ok:
		return "no git"
	case !g.upstream:
		return g.branch + " · local"
	}
	s := g.branch
	if g.ahead > 0 {
		s += " ↑" + strconv.Itoa(g.ahead)
	}
	if g.behind > 0 {
		s += " ↓" + strconv.Itoa(g.behind)
	}
	return s
}
