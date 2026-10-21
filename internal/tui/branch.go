package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/project"
)

// branchRows ist die Höhe der Liste im Fenster „Ziel-Branch“.
const branchRows = 10

// branchPicker wählt den Ziel-Branch eines Projekts. path ist der absolute Projektpfad.
type branchPicker struct {
	path, name string
	branches   []string
	current    string
	cursor     int
}

// openBranchPicker zeigt die lokalen Branches des gewählten Projekts. Vorgewählt ist der
// aktuelle Ziel-Branch.
func (m Model) openBranchPicker() (Model, tea.Cmd) {
	p := m.selectedProject()
	if p == nil {
		return m, nil
	}
	idx := m.configIndex(p.Path)
	if idx < 0 {
		m.setMessage("Projekt nicht in der Konfiguration gefunden", true)
		return m, nil
	}
	branches, err := localBranches(p.Path)
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	if len(branches) == 0 {
		m.setMessage("Das Projekt hat keine lokalen Branches", true)
		return m, nil
	}
	current := m.cfg.Projects[idx].Base()
	m.branch = &branchPicker{path: p.Path, name: p.Name(), branches: branches, current: current,
		cursor: max(0, slices.Index(branches, current))}
	m.mode = modeBranch
	return m, nil
}

// localBranches liefert die lokalen Branches ohne die PRD-Branches unter feat/.
func localBranches(dir string) ([]string, error) {
	out, err := git.Run(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, b := range git.Lines(out) {
		if !strings.HasPrefix(b, project.FeatureBranchPrefix) {
			branches = append(branches, b)
		}
	}
	return branches, nil
}

func (m Model) updateBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	b := m.branch
	switch msg.String() {
	case "esc", "ctrl+c", "q":
		m.mode = modeNormal
		m.branch = nil
	case "j", "down":
		b.cursor = min(b.cursor+1, len(b.branches)-1)
	case "k", "up":
		b.cursor = max(b.cursor-1, 0)
	case "enter":
		m.mode = modeNormal
		m.branch = nil
		return m.setBaseBranch(b.path, b.branches[b.cursor])
	}
	return m, nil
}

// setBaseBranch schreibt den Ziel-Branch in die Konfiguration. Der Standard steht dort nicht,
// damit config.yaml knapp bleibt. Laufende PRDs behalten den Ziel-Branch ihres Starts.
func (m Model) setBaseBranch(path, branch string) (Model, tea.Cmd) {
	idx := m.configIndex(path)
	if idx < 0 {
		m.setMessage("Projekt nicht in der Konfiguration gefunden", true)
		return m, nil
	}
	old := m.cfg.Projects[idx].BaseBranch
	m.cfg.Projects[idx].BaseBranch = branch
	if branch == config.DefaultBaseBranch {
		m.cfg.Projects[idx].BaseBranch = ""
	}
	if err := m.cfg.Save(m.cfgPath); err != nil {
		m.cfg.Projects[idx].BaseBranch = old
		m.setMessage(err.Error(), true)
		return m, nil
	}
	m.setMessage(fmt.Sprintf("Ziel-Branch: %s. Neue Läufe zweigen davon ab und mergen dorthin.", branch), false)
	m.loading = true
	return m, m.loadProjects(path)
}

// viewBranchPicker zeigt höchstens branchRows Branches um die Auswahl herum.
func (m Model) viewBranchPicker() string {
	b := m.branch
	lines := []string{styleMuted.Render("Worktrees zweigen davon ab, der Merge geht dorthin."), ""}
	top := min(max(0, b.cursor-branchRows/2), max(0, len(b.branches)-branchRows))
	for i := top; i < min(top+branchRows, len(b.branches)); i++ {
		label := b.branches[i]
		if label == b.current {
			label += " (aktuell)"
		}
		if i == b.cursor {
			lines = append(lines, styleSelected.Render("> "+label))
		} else {
			lines = append(lines, styleMuted.Render("  "+label))
		}
	}
	w, _ := m.dialogSize()
	return dialog("Ziel-Branch · "+b.name, lines, w, len(lines)+4)
}
