package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/fsutil"
	"lazydev/internal/repo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The wizard edits .lazydev/ of the selected project: base branch, knowledge files, steps per file,
// prompts and a summary. Adding a project creates .lazydev/, and the wizard brings back missing
// files. It writes baseBranch and knowledge only and keeps other fields, order and comments of
// config.yaml.

const (
	pageBranch = iota
	pageKnowledge
	pageSteps
	pagePrompts
	pageSummary
	wizardPages
)

const (
	// wizardRows is the height of the lists in the wizard.
	wizardRows = 10
	// wizardWidth is the outer width of the wizard box.
	wizardWidth = 72
)

// preselectedKnowledge are marked if they exist and config.yaml has no knowledge yet.
var preselectedKnowledge = []string{"CLAUDE.md", "ARCHITECTURE.md", "_ARCHITECTURE.md"}

// wizard is the state of the open wizard. steps holds per file the steps in the order of
// repo.Steps.
type wizard struct {
	path, name string
	page       int

	branches     []string
	branchCursor int
	// base is the base branch of config.yaml, used without local branches.
	base string

	files      []string
	fileCursor int
	marked     map[string]bool
	steps      map[string][]bool
	stepCursor int
	// untracked are files from knowledge that are no candidates.
	untracked map[string]bool

	// prompts is true per step whose .lazydev/<step>.md exists.
	prompts      []bool
	promptCursor int
	// confirmReset is true while the question to overwrite a prompt is open.
	confirmReset bool
	// editor is the terminal that edits a prompt. The wizard hides until it ends, 0 without one.
	editor int

	// preview is config.yaml as saving writes it, changed is false if it equals the file.
	preview []byte
	changed bool
	err     error
}

// openWizard opens the wizard for the selected project. It first adds missing files to
// .lazydev/. A wizard that waits for its editor comes back with its choices.
func (m *model) openWizard() {
	p := m.current()
	if p == nil {
		m.err = "Add a folder first: a in the left column"
		return
	}
	if w := m.wizard; w != nil && w.path == p.path {
		w.editor = 0
		w.refreshPrompts()
		return
	}
	if err := repo.Init(p.path); err != nil {
		m.err = err.Error()
		return
	}
	m.readLazydev()
	if p.lazydev.Err != nil {
		m.err = p.lazydev.Err.Error()
		return
	}
	m.wizard = newWizard(p.path, p.lazydev.Config)
}

func newWizard(dir string, cfg repo.Config) *wizard {
	w := &wizard{path: dir, name: filepath.Base(dir), base: cfg.Base(),
		marked: map[string]bool{}, steps: map[string][]bool{}, untracked: map[string]bool{}}
	// Folders without git have no branches and no candidates, the wizard still edits the rest.
	w.branches, _ = repo.LocalBranches(dir)
	w.branchCursor = max(0, slices.Index(w.branches, w.base))
	w.files, _ = repo.KnowledgeCandidates(dir)

	for _, key := range repo.KnowledgeKeys {
		for _, f := range cfg.Knowledge[key] {
			if w.marked[f] {
				continue
			}
			if !slices.Contains(w.files, f) {
				w.files = append(w.files, f)
				w.untracked[f] = true
			}
			w.marked[f] = true
			w.steps[f] = make([]bool, len(repo.Steps))
		}
	}
	for f, s := range w.steps {
		all := slices.Contains(cfg.Knowledge[repo.KnowledgeAll], f)
		for i, step := range repo.Steps {
			s[i] = all || slices.Contains(cfg.Knowledge[step], f)
		}
	}
	if len(cfg.Knowledge) == 0 {
		for _, f := range preselectedKnowledge {
			if slices.Contains(w.files, f) {
				w.toggle(f)
			}
		}
	}
	w.refreshPrompts()
	return w
}

// selectedBranch is the chosen base branch, the configured one without local branches.
func (w *wizard) selectedBranch() string {
	if len(w.branches) == 0 {
		return w.base
	}
	return w.branches[w.branchCursor]
}

// toggle marks f or removes the mark. Newly marked files apply to create and implement.
func (w *wizard) toggle(f string) {
	w.marked[f] = !w.marked[f]
	if _, ok := w.steps[f]; !ok {
		w.steps[f] = []bool{true, true, false, false}
	}
}

// markedFiles lists the marked files in list order.
func (w *wizard) markedFiles() []string {
	var out []string
	for _, f := range w.files {
		if w.marked[f] {
			out = append(out, f)
		}
	}
	return out
}

// knowledge builds the knowledge section. A file marked for all four steps goes under all.
func (w *wizard) knowledge() map[string][]string {
	k := map[string][]string{}
	for _, f := range w.markedFiles() {
		s := w.steps[f]
		if !slices.Contains(s, false) {
			k[repo.KnowledgeAll] = append(k[repo.KnowledgeAll], f)
			continue
		}
		for i, on := range s {
			if on {
				k[repo.Steps[i]] = append(k[repo.Steps[i]], f)
			}
		}
	}
	return k
}

func (w *wizard) refreshPrompts() {
	w.prompts = w.prompts[:0]
	for _, step := range repo.Steps {
		_, err := os.Stat(w.promptPath(step))
		w.prompts = append(w.prompts, err == nil)
	}
}

func (w *wizard) promptPath(step string) string {
	return repo.PromptPath(w.path, step)
}

// lastChoicePage is the page before the prompts: steps if files are marked, otherwise knowledge.
func (w *wizard) lastChoicePage() int {
	if len(w.markedFiles()) > 0 {
		return pageSteps
	}
	return pageKnowledge
}

// updateWizard handles all keys while the wizard is visible.
func (m model) updateWizard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := m.wizard
	key := msg.String()
	if w.confirmReset {
		w.confirmReset = false
		if key == "y" || key == "j" {
			m.writePrompt(repo.Steps[w.promptCursor])
		}
		return m, nil
	}
	if key == "esc" || key == "ctrl+c" {
		m.wizard = nil
		return m, nil
	}
	switch w.page {
	case pageBranch:
		switch key {
		case "j", "down":
			w.branchCursor = clamp(w.branchCursor+1, len(w.branches))
		case "k", "up":
			w.branchCursor = clamp(w.branchCursor-1, len(w.branches))
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
			w.stepCursor = 0
			w.page = pagePrompts
			if len(w.markedFiles()) > 0 {
				w.page = pageSteps
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
			s := w.steps[marked[w.stepCursor]]
			s[key[0]-'1'] = !s[key[0]-'1']
		case "enter":
			w.page = pagePrompts
		case "backspace":
			w.page = pageKnowledge
		}
	case pagePrompts:
		w.refreshPrompts()
		switch key {
		case "j", "down":
			w.promptCursor = clamp(w.promptCursor+1, len(repo.Steps))
		case "k", "up":
			w.promptCursor = clamp(w.promptCursor-1, len(repo.Steps))
		case "o":
			m.editPrompt()
		case "r":
			// A missing prompt comes back right away, an existing one only after the question.
			if w.prompts[w.promptCursor] {
				w.confirmReset = true
			} else {
				m.writePrompt(repo.Steps[w.promptCursor])
			}
		case "enter":
			// .lazydev/ may be gone since the wizard opened. Init brings it back first.
			w.err = repo.Init(w.path)
			if w.err == nil {
				w.preview, w.changed, w.err = repo.Settings(w.path, w.selectedBranch(), w.knowledge())
			}
			w.page = pageSummary
		case "backspace":
			w.page = w.lastChoicePage()
		}
	case pageSummary:
		switch key {
		case "enter":
			m.saveWizard()
		case "backspace":
			w.page = pagePrompts
		}
	}
	return m, nil
}

// writePrompt writes the prompt of the simple workflow for step into .lazydev/, which it creates
// again if it is gone.
func (m *model) writePrompt(step string) {
	err := repo.Init(m.wizard.path)
	if err == nil {
		err = repo.WritePrompt(m.wizard.path, step)
	}
	if err != nil {
		m.err = err.Error()
	}
	m.wizard.refreshPrompts()
	m.readLazydev()
}

// editPrompt opens the selected prompt in the editor as a center tab and hides the wizard until
// the editor ends. A missing prompt starts as the one of the simple workflow.
func (m *model) editPrompt() {
	w := m.wizard
	step := repo.Steps[w.promptCursor]
	if !w.prompts[w.promptCursor] {
		m.writePrompt(step)
	}
	m.err = ""
	m.openTerminal(editorCommand(w.promptPath(step)))
	if m.err == "" {
		w.editor = m.nextID
	}
}

// saveWizard writes config.yaml and closes the wizard. lazydev does not commit it.
func (m *model) saveWizard() {
	w := m.wizard
	switch {
	case w.err != nil:
		m.err = w.err.Error()
		return
	case w.changed:
		if err := fsutil.WriteAtomic(repo.ConfigPath(w.path), w.preview, 0o644); err != nil {
			m.err = err.Error()
			return
		}
		m.note = repo.Dir + "/config.yaml saved. Commit it, so the team gets it."
	default:
		m.note = "No change to " + repo.Dir + "/config.yaml."
	}
	m.wizard = nil
	m.readLazydev()
}

// viewWizard renders the current page as a box for the overlay.
func (m model) viewWizard() string {
	w := m.wizard
	t := themes[m.theme]
	text := lipgloss.NewStyle().Foreground(t.text)
	muted := text.Faint(true)
	inner := min(wizardWidth, m.width-4) - 4
	line := func(selected bool, s string) string {
		s = pad(s, inner-2)
		if selected {
			return text.Bold(true).Foreground(t.selectedFg).Background(t.selectedBg).Render("> " + s)
		}
		return text.Render("  " + s)
	}
	var lines []string
	heading := func(page int, title, hint string) {
		lines = append(lines, text.Bold(true).Foreground(t.focus).Render(fmt.Sprintf("%d/%d %s", page+1, wizardPages, title)),
			muted.Render(ansi.Truncate(hint, inner, "…")), "")
	}
	switch w.page {
	case pageBranch:
		heading(pageBranch, "Base branch", "Worktrees branch off it, merges go there.")
		if len(w.branches) == 0 {
			lines = append(lines, muted.Render("No local branches, "+w.base+" stays."))
		}
		start, end := scrollWindow(w.branchCursor, len(w.branches), wizardRows)
		for i := start; i < end; i++ {
			label := w.branches[i]
			if label == w.base {
				label += " (current)"
			}
			lines = append(lines, line(i == w.branchCursor, label))
		}
	case pageKnowledge:
		heading(pageKnowledge, "Knowledge files", "Claude reads marked files before anything else.")
		if len(w.files) == 0 {
			lines = append(lines, muted.Render("No tracked .md files outside prd/."))
		}
		start, end := scrollWindow(w.fileCursor, len(w.files), wizardRows)
		for i := start; i < end; i++ {
			f := w.files[i]
			label := "[ ] " + f
			if w.marked[f] {
				label = "[x] " + f
			}
			if w.untracked[f] {
				label += " (not tracked)"
			}
			lines = append(lines, line(i == w.fileCursor, label))
		}
	case pageSteps:
		heading(pageSteps, "Steps per file", "1 create · 2 implement · 3 review · 4 merge")
		marked := w.markedFiles()
		fileW := 0
		for _, f := range marked {
			fileW = max(fileW, ansi.StringWidth(f))
		}
		fileW = min(fileW, inner-2-4*4-2)
		start, end := scrollWindow(w.stepCursor, len(marked), wizardRows)
		for i := start; i < end; i++ {
			f := marked[i]
			fields := make([]string, len(repo.Steps))
			for j, step := range repo.Steps {
				fields[j] = "[ ]"
				if w.steps[f][j] {
					fields[j] = "[" + step[:1] + "]"
				}
			}
			lines = append(lines, line(i == w.stepCursor, pad(f, fileW)+"  "+strings.Join(fields, " ")))
		}
	case pagePrompts:
		heading(pagePrompts, "Prompts", "Edits apply right away. The workflow reads only these.")
		for i, step := range repo.Steps {
			state := "missing"
			if w.prompts[i] {
				state = "present"
			}
			lines = append(lines, line(i == w.promptCursor, pad(step, 10)+pad(repo.Dir+"/"+step+".md", 24)+state))
		}
		if w.confirmReset {
			lines = append(lines, "", text.Bold(true).Render("Overwrite with the simple workflow? (y/n)"))
		}
	case pageSummary:
		heading(pageSummary, "Summary", "Enter saves. lazydev does not commit.")
		switch {
		case w.err != nil:
			lines = append(lines, text.Foreground(t.focus).Render(ansi.Truncate(w.err.Error(), inner, "…")))
		case !w.changed:
			lines = append(lines, muted.Render(repo.Dir+"/config.yaml stays unchanged."))
		default:
			lines = append(lines, text.Render(repo.Dir+"/config.yaml:"))
			for _, l := range strings.Split(strings.TrimRight(string(w.preview), "\n"), "\n") {
				lines = append(lines, text.Render(ansi.Truncate("  "+l, inner, "…")))
			}
		}
	}
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	lines = append([]string{""}, append(lines, "")...)
	return box(repo.Dir+"/ · "+w.name, strings.Join(lines, "\n"), inner+4, len(lines)+2, t.focus, true, t)
}

// wizardKeys is the key help of the current page.
func (w *wizard) keys() string {
	switch {
	case w.confirmReset:
		return " y overwrite · n keep"
	case w.page == pageBranch:
		return " j/k branch · enter next · esc cancel"
	case w.page == pageKnowledge:
		return " j/k file · space mark · enter next · backspace back · esc cancel"
	case w.page == pageSteps:
		return " j/k file · 1-4 toggle step · enter next · backspace back · esc cancel"
	case w.page == pagePrompts:
		return " j/k prompt · o edit · r reset to simple workflow · enter next · backspace back · esc cancel"
	}
	return " enter save · backspace back · esc cancel"
}

// scrollWindow is the visible part of a list of total rows that keeps cursor in the middle.
func scrollWindow(cursor, total, rows int) (start, end int) {
	if total <= rows {
		return 0, total
	}
	start = max(0, min(cursor-rows/2, total-rows))
	return start, start + rows
}
