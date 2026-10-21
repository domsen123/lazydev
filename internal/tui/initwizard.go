package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/git"
	"lazydev/internal/herdr"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
)

// Seiten des Wizards. pagePrompts gibt es nur im Modus „bearbeiten“.
const (
	pageBranch = iota
	pageKnowledge
	pageSteps
	pagePrompts
	pageSummary
)

// knowledgeRows ist die Höhe der Dateiliste auf der Seite Knowledge-Dateien.
const knowledgeRows = 10

// preselectedKnowledge sind die Dateien, die der Wizard vormarkiert, falls vorhanden.
var preselectedKnowledge = []string{"CLAUDE.md", "ARCHITECTURE.md", "_ARCHITECTURE.md"}

// initWizard legt .lazydev/ im Projekt path an oder bearbeitet es (edit). steps hält je Datei die
// Schritte in der Reihenfolge von prompts.Steps.
type initWizard struct {
	path, name string
	edit       bool
	page       int
	branch     *branchPicker
	files      []string
	fileCursor int
	marked     map[string]bool
	steps      map[string][]bool
	stepCursor int
	// untracked sind Dateien aus knowledge, die keine Kandidaten sind.
	untracked map[string]bool
	// promptCursor und states gehören zur Seite Prompts, states in der Reihenfolge von prompts.Steps.
	promptCursor int
	states       []prompts.FileState
	// current ist die geladene .lazydev/config.yaml im Modus „bearbeiten“.
	current config.Repo
	// result ist die config.yaml der Zusammenfassung.
	result config.Repo
}

// wizardEditedMsg meldet das Ende des Editors auf der Seite Prompts ohne Herdr.
type wizardEditedMsg struct {
	err error
}

// openInitWizard öffnet den Wizard für das gewählte Projekt. Ohne .lazydev/ legt er den Ordner an,
// sonst bearbeitet er ihn.
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
		return m.openEditWizard(p)
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
	w := newInitWizard(p, b, files)
	for _, f := range preselectedKnowledge {
		if slices.Contains(files, f) {
			w.toggle(f)
		}
	}
	m.wizard = w
	m.mode = modeInit
	return m, nil
}

// openEditWizard öffnet den Wizard im Modus „bearbeiten“, vorbelegt aus .lazydev/config.yaml.
func (m Model) openEditWizard(p *project.Project) (Model, tea.Cmd) {
	repo, _, err := config.LoadRepo(p.Path)
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	b, err := newBranchPicker(p, repo.Base())
	if err != nil {
		m.setMessage(err.Error(), true)
		return m, nil
	}
	files, err := knowledgeCandidates(p.Path)
	if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	w := newInitWizard(p, b, files)
	w.edit, w.current = true, repo
	for _, key := range config.KnowledgeKeys {
		for _, f := range repo.Knowledge[key] {
			if w.marked[f] {
				continue
			}
			if !slices.Contains(w.files, f) {
				w.files = append(w.files, f)
				w.untracked[f] = true
			}
			w.marked[f] = true
			w.steps[f] = make([]bool, len(prompts.Steps))
		}
	}
	for f, s := range w.steps {
		all := slices.Contains(repo.Knowledge[config.KnowledgeAll], f)
		for i, step := range prompts.Steps {
			s[i] = all || slices.Contains(repo.Knowledge[string(step)], f)
		}
	}
	if err := w.refreshStates(); err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	m.wizard = w
	m.mode = modeInit
	return m, nil
}

func newInitWizard(p *project.Project, b *branchPicker, files []string) *initWizard {
	return &initWizard{path: p.Path, name: p.Name(), branch: b, files: files, marked: map[string]bool{},
		steps: map[string][]bool{}, untracked: map[string]bool{}}
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

// refreshStates bestimmt die Zustände der Prompt-Dateien neu.
func (w *initWizard) refreshStates() error {
	states := make([]prompts.FileState, len(prompts.Steps))
	for i, step := range prompts.Steps {
		s, err := prompts.StateOf(w.path, step)
		if err != nil {
			return err
		}
		states[i] = s
	}
	w.states = states
	return nil
}

// knowledge baut knowledge aus den Seiten. Eine Datei, die bei allen vier Schritten markiert ist,
// steht unter all.
func (w *initWizard) knowledge() map[string][]string {
	var k map[string][]string
	add := func(key, f string) {
		if k == nil {
			k = map[string][]string{}
		}
		k[key] = append(k[key], f)
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
	return k
}

// repo baut die config.yaml aus den Seiten. Beim Anlegen kommen die übrigen Felder aus den
// Übergangsfeldern p der zentralen config.yaml, beim Bearbeiten aus der geladenen Datei.
func (w *initWizard) repo(p config.Project) config.Repo {
	r := config.Repo{Preset: p.Preset, AllowedTools: p.AllowedTools, Hooks: p.Hooks}
	if w.edit {
		r = w.current
	} else if r.Preset == "" {
		r.Preset = config.DetectPreset(w.path)
	}
	r.BaseBranch = ""
	if b := w.branch.selected(); b != config.DefaultBaseBranch {
		r.BaseBranch = b
	}
	r.Knowledge = w.knowledge()
	return r
}

// afterSteps ist die Seite nach den Schritten je Datei.
func (m Model) afterSteps() (Model, tea.Cmd) {
	if m.wizard.edit {
		m.wizard.page = pagePrompts
		if err := m.wizard.refreshStates(); err != nil {
			m.setMessage(firstLine(err.Error()), true)
		}
		return m, nil
	}
	return m.wizardSummary()
}

// lastChoicePage ist die letzte Seite mit Knowledge-Auswahl: Schritte, wenn Dateien markiert sind,
// sonst Knowledge. Sie liegt vor den Prompts bzw. vor der Zusammenfassung beim Anlegen.
func (w *initWizard) lastChoicePage() int {
	if len(w.markedFiles()) > 0 {
		return pageSteps
	}
	return pageKnowledge
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
				return m.afterSteps()
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
			return m.afterSteps()
		case "backspace":
			w.page = pageKnowledge
		}
	case pagePrompts:
		// In Herdr endet der Editor ohne Nachricht, die Zustände folgen deshalb jedem Tastendruck.
		if err := w.refreshStates(); err != nil {
			m.setMessage(firstLine(err.Error()), true)
			return m, nil
		}
		switch key {
		case "j", "down":
			w.promptCursor = clamp(w.promptCursor+1, len(prompts.Steps))
		case "k", "up":
			w.promptCursor = clamp(w.promptCursor-1, len(prompts.Steps))
		case "o":
			return m.openWizardEditor()
		case "r":
			return m.resetWizardPrompt()
		case "enter":
			return m.wizardSummary()
		case "backspace":
			w.page = w.lastChoicePage()
		}
	case pageSummary:
		switch key {
		case "enter":
			return m.writeInitWizard()
		case "backspace":
			if w.edit {
				w.page = pagePrompts
			} else {
				w.page = w.lastChoicePage()
			}
		}
	}
	return m, nil
}

// openWizardEditor öffnet den Prompt der Auswahl im Editor, gebaut wie openPRDEditor. Fehlt die
// Datei, legt sie lazydev vorher mit dem Standard-Prompt an. Ohne Herdr bestimmt
// handleWizardEdited die Zustände nach dem Editor neu.
func (m Model) openWizardEditor() (Model, tea.Cmd) {
	w := m.wizard
	step := prompts.Steps[w.promptCursor]
	path := filepath.Join(w.path, prompts.FileName(step))
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		err = os.WriteFile(path, []byte(prompts.Default(step)), 0o644)
		if err != nil {
			m.setMessage(firstLine(err.Error()), true)
			return m, nil
		}
		if err := w.refreshStates(); err != nil {
			m.setMessage(firstLine(err.Error()), true)
		}
	} else if err != nil {
		m.setMessage(firstLine(err.Error()), true)
		return m, nil
	}
	args := append(editorCommand(), path)
	if herdr.Available() {
		return m.openInteractive(w.path, args, "Editor")
	}
	return m, execInteractive(w.path, args, func(err error) tea.Msg { return wizardEditedMsg{err} })
}

// handleWizardEdited bestimmt nach dem Editor die Zustände der Seite Prompts neu.
func (m Model) handleWizardEdited(msg wizardEditedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.setMessage("Editor: "+firstLine(msg.err.Error()), true)
	}
	if m.wizard != nil {
		if err := m.wizard.refreshStates(); err != nil {
			m.setMessage(firstLine(err.Error()), true)
		}
	}
	return m, nil
}

// resetWizardPrompt löscht den Prompt der Auswahl nach Rückfrage. Danach gilt der Standard von
// lazydev. Ohne Datei bewirkt es nichts.
func (m Model) resetWizardPrompt() (Model, tea.Cmd) {
	w := m.wizard
	if w.states[w.promptCursor] == prompts.StateDefault {
		return m, nil
	}
	file := prompts.FileName(prompts.Steps[w.promptCursor])
	path := filepath.Join(w.path, file)
	m.returnMode = modeInit
	return m.ask(file+" löschen und den Standard von lazydev nutzen? (j/n)", func(m Model) (Model, tea.Cmd) {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			m.setMessage(firstLine(err.Error()), true)
		}
		if m.wizard != nil {
			if err := m.wizard.refreshStates(); err != nil {
				m.setMessage(firstLine(err.Error()), true)
			}
		}
		return m, nil
	})
}

// wizardSummary baut die config.yaml und zeigt die Zusammenfassung.
func (m Model) wizardSummary() (Model, tea.Cmd) {
	w := m.wizard
	var p config.Project
	if !w.edit {
		idx := m.configIndex(w.path)
		if idx < 0 {
			m.setMessage("Projekt nicht in der Konfiguration gefunden", true)
			return m, nil
		}
		p = m.cfg.Projects[idx]
	}
	w.result = w.repo(p)
	w.page = pageSummary
	return m, nil
}

// writeInitWizard legt .lazydev/ an und entfernt die übernommenen Felder aus der zentralen
// config.yaml. Beim Bearbeiten ändert es nur baseBranch und knowledge in .lazydev/config.yaml.
// lazydev committet nicht.
func (m Model) writeInitWizard() (Model, tea.Cmd) {
	w := m.wizard
	if w.edit {
		m.mode = modeNormal
		m.wizard = nil
		changed, err := config.SetRepoSettings(w.path, w.result.BaseBranch, w.result.Knowledge)
		switch {
		case err != nil:
			m.setMessage(firstLine(err.Error()), true)
			return m, nil
		case !changed:
			m.setMessage("Keine Änderung an "+config.RepoDir+"/config.yaml", false)
			return m, nil
		}
		m.setMessage(config.RepoDir+"/config.yaml gespeichert. Committe die Änderung, damit das Team sie bekommt.", false)
		m.loading = true
		return m, m.loadProjects(w.path)
	}
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

// viewInitWizard zeigt die aktuelle Seite im Fenster „.lazydev/ anlegen“ oder „.lazydev/ bearbeiten“.
func (m Model) viewInitWizard() string {
	w := m.wizard
	var lines []string
	pages, suffix := 4, " anlegen · "
	if w.edit {
		pages, suffix = 5, " bearbeiten · "
	}
	heading := func(n int, title, hint string) {
		lines = append(lines, styleSelected.Render(fmt.Sprintf("%d/%d %s", n, pages, title)), styleMuted.Render(hint), "")
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
			f := w.files[i]
			box := "[ ]"
			if w.marked[f] {
				box = "[x]"
			}
			label := box + " " + f
			if w.untracked[f] {
				label += " (nicht getrackt)"
			}
			lines = append(lines, wizardLine(i == w.fileCursor, label))
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
	case pagePrompts:
		heading(4, "Prompts", "Änderungen an Prompts gelten sofort.")
		stepWidth, fileWidth := 0, 0
		for _, step := range prompts.Steps {
			stepWidth = max(stepWidth, ansi.StringWidth(string(step)))
			fileWidth = max(fileWidth, ansi.StringWidth(prompts.FileName(step)))
		}
		for i, step := range prompts.Steps {
			text := padRight(string(step), stepWidth) + "  " + padRight(prompts.FileName(step), fileWidth) + "  " + w.stateLabel(i)
			lines = append(lines, wizardLine(i == w.promptCursor, text))
		}
	case pageSummary:
		if w.edit {
			heading(5, "Zusammenfassung", "Enter speichert. lazydev committet nicht.")
		} else {
			heading(4, "Zusammenfassung", "Enter legt die Dateien an. lazydev committet nicht.")
		}
		lines = append(lines, config.RepoDir+"/config.yaml:")
		data, err := w.result.Marshal()
		if err != nil {
			lines = append(lines, styleError.Render(err.Error()))
		}
		for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			lines = append(lines, "  "+l)
		}
		if w.edit {
			lines = append(lines, "", "Prompts:")
			for i, step := range prompts.Steps {
				lines = append(lines, "  "+string(step)+": "+w.stateLabel(i))
			}
		} else {
			lines = append(lines, "", "lazydev legt an:")
			for _, f := range prompts.InitFiles() {
				lines = append(lines, "  "+f)
			}
		}
	}
	width, height := m.dialogSize()
	return dialog(config.RepoDir+"/"+suffix+w.name, lines, width, min(height, len(lines)+4))
}

// stateLabel liefert den Zustand des Prompts an Position i in prompts.Steps.
func (w *initWizard) stateLabel(i int) string {
	if i >= len(w.states) {
		return ""
	}
	return prompts.Label(w.states[i])
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
	case pagePrompts:
		return [][2]string{{"↑ ↓", "wählen"}, {"o", "Editor"}, {"r", "Standard"}, {"Enter", "weiter"}, {"Backspace", "zurück"}, {"Esc", "abbrechen"}}
	}
	if w.edit {
		return [][2]string{{"Enter", "speichern"}, {"Backspace", "zurück"}, {"Esc", "abbrechen"}}
	}
	return [][2]string{{"Enter", "anlegen"}, {"Backspace", "zurück"}, {"Esc", "abbrechen"}}
}
