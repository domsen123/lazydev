package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
)

// Seiten des Wizards „.lazydev/ anlegen“.
const (
	pageBranch = iota
	pageKnowledge
	pageSteps
	pageSummary
)

// knowledgeRows ist die Höhe der Dateiliste auf der Seite Knowledge-Dateien.
const knowledgeRows = 10

// preselectedKnowledge sind die Dateien, die der Wizard vormarkiert, falls vorhanden.
var preselectedKnowledge = []string{"CLAUDE.md", "ARCHITECTURE.md", "_ARCHITECTURE.md"}

// initWizard legt .lazydev/ im Projekt path an. steps hält je Datei die Schritte in der
// Reihenfolge von prompts.Steps.
type initWizard struct {
	path, name string
	page       int
	branch     *branchPicker
	files      []string
	fileCursor int
	marked     map[string]bool
	steps      map[string][]bool
	stepCursor int
	// result ist die config.yaml der Zusammenfassung.
	result config.Repo
}

// openInitWizard öffnet den Wizard für das gewählte Projekt, wenn es noch kein .lazydev/ hat.
func (m Model) openInitWizard() (Model, tea.Cmd) {
	p := m.selectedProject()
	if p == nil {
		return m, nil
	}
	idx := m.configIndex(p.Path)
	if idx < 0 {
		m.setMessage("Projekt nicht in der Konfiguration gefunden", true)
		return m, nil
	}
	if _, err := os.Lstat(filepath.Join(p.Path, config.RepoDir)); err == nil {
		m.setMessage(config.RepoDir+"/ existiert schon. Bearbeite die Dateien im Editor.", true)
		return m, nil
	}
	current := m.cfg.Projects[idx].BaseBranch
	if current == "" {
		current = p.Branch
	}
	b, err := newBranchPicker(p, current)
	if err != nil {
		m.setMessage(err.Error(), true)
		return m, nil
	}
	// Im Wizard gibt es noch keinen Ziel-Branch, den „(aktuell)“ markieren könnte.
	b.current = ""
	files, err := knowledgeCandidates(p.Path)
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	w := &initWizard{path: p.Path, name: p.Name(), branch: b, files: files, marked: map[string]bool{}, steps: map[string][]bool{}}
	for _, f := range preselectedKnowledge {
		if slices.Contains(files, f) {
			w.toggle(f)
		}
	}
	m.wizard = w
	m.mode = modeInit
	return m, nil
}

// knowledgeCandidates listet die getrackten .md-Dateien ohne prd/ und .lazydev/, sortiert. Nur
// getrackte Dateien gibt es auch im Worktree.
func knowledgeCandidates(dir string) ([]string, error) {
	out, err := git.Run(dir, "-c", "core.quotePath=false", "ls-files", "--", "*.md")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range git.Lines(out) {
		if strings.HasPrefix(f, project.PRDDir+"/") || strings.HasPrefix(f, config.RepoDir+"/") {
			continue
		}
		files = append(files, f)
	}
	slices.Sort(files)
	return files, nil
}

// toggle markiert f oder hebt die Markierung auf. Neu markierte Dateien gelten für create und
// implement.
func (w *initWizard) toggle(f string) {
	w.marked[f] = !w.marked[f]
	if _, ok := w.steps[f]; !ok {
		w.steps[f] = []bool{true, true, false, false}
	}
}

// markedFiles liefert die markierten Dateien in der Reihenfolge der Liste.
func (w *initWizard) markedFiles() []string {
	var out []string
	for _, f := range w.files {
		if w.marked[f] {
			out = append(out, f)
		}
	}
	return out
}

// repo baut die config.yaml aus den Seiten und den Übergangsfeldern p der zentralen config.yaml.
// Eine Datei, die bei allen vier Schritten markiert ist, steht unter all.
func (w *initWizard) repo(p config.Project) config.Repo {
	r := config.Repo{Preset: p.Preset, AllowedTools: p.AllowedTools, Hooks: p.Hooks}
	if r.Preset == "" {
		r.Preset = config.DetectPreset(w.path)
	}
	if b := w.branch.selected(); b != config.DefaultBaseBranch {
		r.BaseBranch = b
	}
	add := func(key, f string) {
		if r.Knowledge == nil {
			r.Knowledge = map[string][]string{}
		}
		r.Knowledge[key] = append(r.Knowledge[key], f)
	}
	for _, f := range w.markedFiles() {
		s := w.steps[f]
		if !slices.Contains(s, false) {
			add(config.KnowledgeAll, f)
			continue
		}
		for i, on := range s {
			if on {
				add(string(prompts.Steps[i]), f)
			}
		}
	}
	return r
}

func (m Model) updateInitWizard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := m.wizard
	key := msg.String()
	if key == "esc" || key == "ctrl+c" {
		m.mode = modeNormal
		m.wizard = nil
		return m, nil
	}
	switch w.page {
	case pageBranch:
		switch key {
		case "j", "down":
			w.branch.move(1)
		case "k", "up":
			w.branch.move(-1)
		case "enter":
			w.page = pageKnowledge
		}
	case pageKnowledge:
		switch key {
		case "j", "down":
			w.fileCursor = clamp(w.fileCursor+1, len(w.files))
		case "k", "up":
			w.fileCursor = clamp(w.fileCursor-1, len(w.files))
		case " ", "space":
			if w.fileCursor < len(w.files) {
				w.toggle(w.files[w.fileCursor])
			}
		case "enter":
			if len(w.markedFiles()) > 0 {
				w.stepCursor = 0
				w.page = pageSteps
			} else {
				return m.wizardSummary()
			}
		case "backspace":
			w.page = pageBranch
		}
	case pageSteps:
		marked := w.markedFiles()
		switch key {
		case "j", "down":
			w.stepCursor = clamp(w.stepCursor+1, len(marked))
		case "k", "up":
			w.stepCursor = clamp(w.stepCursor-1, len(marked))
		case "1", "2", "3", "4":
			i := int(key[0] - '1')
			f := marked[w.stepCursor]
			w.steps[f][i] = !w.steps[f][i]
		case "enter":
			return m.wizardSummary()
		case "backspace":
			w.page = pageKnowledge
		}
	case pageSummary:
		switch key {
		case "enter":
			return m.writeInitWizard()
		case "backspace":
			w.page = pageKnowledge
			if len(w.markedFiles()) > 0 {
				w.page = pageSteps
			}
		}
	}
	return m, nil
}

// wizardSummary baut die config.yaml und zeigt die Zusammenfassung.
func (m Model) wizardSummary() (Model, tea.Cmd) {
	idx := m.configIndex(m.wizard.path)
	if idx < 0 {
		m.setMessage("Projekt nicht in der Konfiguration gefunden", true)
		return m, nil
	}
	m.wizard.result = m.wizard.repo(m.cfg.Projects[idx])
	m.wizard.page = pageSummary
	return m, nil
}

// writeInitWizard legt .lazydev/ an und entfernt die übernommenen Felder aus der zentralen
// config.yaml. lazydev committet nicht.
func (m Model) writeInitWizard() (Model, tea.Cmd) {
	w := m.wizard
	idx := m.configIndex(w.path)
	if idx < 0 {
		m.setMessage("Projekt nicht in der Konfiguration gefunden", true)
		return m, nil
	}
	m.mode = modeNormal
	m.wizard = nil
	if err := prompts.Init(w.path, w.result); err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	old := m.cfg.Projects[idx]
	m.cfg.RemoveLocalProjectSettings(idx)
	if err := m.cfg.Save(m.cfgPath); err != nil {
		m.cfg.Projects[idx] = old
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	m.setMessage(config.RepoDir+"/ angelegt. Committe den Ordner, damit das Team ihn bekommt.", false)
	m.loading = true
	return m, m.loadProjects(w.path)
}

// viewInitWizard zeigt die aktuelle Seite im Fenster „.lazydev/ anlegen“.
func (m Model) viewInitWizard() string {
	w := m.wizard
	var lines []string
	heading := func(n int, title, hint string) {
		lines = append(lines, styleSelected.Render(fmt.Sprintf("%d/4 %s", n, title)), styleMuted.Render(hint), "")
	}
	switch w.page {
	case pageBranch:
		heading(1, "Ziel-Branch", "Worktrees zweigen davon ab, der Merge geht dorthin.")
		lines = append(lines, w.branch.lines()...)
	case pageKnowledge:
		heading(2, "Knowledge-Dateien", "Claude liest markierte Dateien, bevor es etwas anderes tut.")
		if len(w.files) == 0 {
			lines = append(lines, styleMuted.Render("Keine getrackten .md-Dateien außerhalb von prd/"))
		}
		start, end := scrollWindow(w.fileCursor, len(w.files), knowledgeRows)
		for i := start; i < end; i++ {
			box := "[ ]"
			if w.marked[w.files[i]] {
				box = "[x]"
			}
			lines = append(lines, wizardLine(i == w.fileCursor, box+" "+w.files[i]))
		}
	case pageSteps:
		heading(3, "Schritte je Datei", "1 create · 2 implement · 3 review · 4 merge")
		marked := w.markedFiles()
		width := 0
		for _, f := range marked {
			width = max(width, ansi.StringWidth(f))
		}
		start, end := scrollWindow(w.stepCursor, len(marked), knowledgeRows)
		for i := start; i < end; i++ {
			f := marked[i]
			fields := make([]string, len(prompts.Steps))
			for j, step := range prompts.Steps {
				fields[j] = "[ ]"
				if w.steps[f][j] {
					fields[j] = "[" + string(step)[:1] + "]"
				}
			}
			lines = append(lines, wizardLine(i == w.stepCursor, padRight(f, width)+"  "+strings.Join(fields, " ")))
		}
	case pageSummary:
		heading(4, "Zusammenfassung", "Enter legt die Dateien an. lazydev committet nicht.")
		lines = append(lines, config.RepoDir+"/config.yaml:")
		data, err := w.result.Marshal()
		if err != nil {
			lines = append(lines, styleError.Render(err.Error()))
		}
		for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			lines = append(lines, "  "+l)
		}
		lines = append(lines, "", "lazydev legt an:")
		for _, f := range prompts.InitFiles() {
			lines = append(lines, "  "+f)
		}
	}
	width, height := m.dialogSize()
	return dialog(config.RepoDir+"/ anlegen · "+w.name, lines, width, min(height, len(lines)+4))
}

// wizardLine hebt die gewählte Zeile hervor.
func wizardLine(selected bool, text string) string {
	if selected {
		return styleSelected.Render("> " + text)
	}
	return styleMuted.Render("  " + text)
}

// keys liefert die Tasten der Fußzeile für die aktuelle Seite.
func (w *initWizard) keys() [][2]string {
	if w == nil {
		return nil
	}
	switch w.page {
	case pageBranch:
		return [][2]string{{"↑ ↓", "Branch wählen"}, {"Enter", "weiter"}, {"Esc", "abbrechen"}}
	case pageKnowledge:
		return [][2]string{{"↑ ↓", "Datei"}, {"Leertaste", "markieren"}, {"Enter", "weiter"}, {"Backspace", "zurück"}, {"Esc", "abbrechen"}}
	case pageSteps:
		return [][2]string{{"↑ ↓", "Datei"}, {"1-4", "Schritt umschalten"}, {"Enter", "weiter"}, {"Backspace", "zurück"}, {"Esc", "abbrechen"}}
	}
	return [][2]string{{"Enter", "anlegen"}, {"Backspace", "zurück"}, {"Esc", "abbrechen"}}
}
