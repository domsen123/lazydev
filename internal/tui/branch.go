package tui

import (
	"errors"
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
// aktuelle Ziel-Branch. Ohne .lazydev/config.yaml gibt es nichts zu schreiben.
func (m Model) openBranchPicker() (Model, tea.Cmd) {
	p := m.selectedProject()
	if p == nil {
		return m, nil
	}
	repo, exists, err := config.LoadRepo(p.Path)
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	if !exists {
		m.setMessage("Kein "+config.RepoDir+"/ im Projekt. Mit i anlegen.", true)
		return m, nil
	}
	b, err := newBranchPicker(p, repo.Base())
	if err != nil {
		m.setMessage(err.Error(), true)
		return m, nil
	}
	m.branch = b
	m.mode = modeBranch
	return m, nil
}

// newBranchPicker liest die lokalen Branches von p. Vorgewählt ist current.
func newBranchPicker(p *project.Project, current string) (*branchPicker, error) {
	branches, err := localBranches(p.Path)
	if err != nil {
		return nil, errors.New(firstLine(err.Error()))
	}
	if len(branches) == 0 {
		return nil, errors.New("Das Projekt hat keine lokalen Branches")
	}
	return &branchPicker{path: p.Path, name: p.Name(), branches: branches, current: current,
		cursor: max(0, slices.Index(branches, current))}, nil
}

// move verschiebt die Auswahl um delta Zeilen.
func (b *branchPicker) move(delta int) {
	b.cursor = clamp(b.cursor+delta, len(b.branches))
}

// selected liefert den gewählten Branch.
func (b *branchPicker) selected() string {
	return b.branches[b.cursor]
}

// lines zeigt höchstens branchRows Branches um die Auswahl herum.
func (b *branchPicker) lines() []string {
	var lines []string
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
	return lines
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
		b.move(1)
	case "k", "up":
		b.move(-1)
	case "enter":
		m.mode = modeNormal
		m.branch = nil
		return m.setBaseBranch(b.path, b.selected())
	}
	return m, nil
}

// setBaseBranch schreibt den Ziel-Branch in .lazydev/config.yaml. Der Standard steht dort nicht.
// Laufende PRDs behalten den Ziel-Branch ihres Starts.
func (m Model) setBaseBranch(path, branch string) (Model, tea.Cmd) {
	if err := config.SetRepoBaseBranch(path, branch); err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	m.setMessage(fmt.Sprintf("Ziel-Branch: %s. Neue Läufe zweigen davon ab und mergen dorthin.", branch), false)
	m.loading = true
	return m, m.loadProjects(path)
}

// viewBranchPicker zeigt das Fenster „Ziel-Branch“.
func (m Model) viewBranchPicker() string {
	b := m.branch
	lines := append([]string{styleMuted.Render("Worktrees zweigen davon ab, der Merge geht dorthin."), ""}, b.lines()...)
	w, _ := m.dialogSize()
	return dialog("Ziel-Branch · "+b.name, lines, w, len(lines)+4)
}
