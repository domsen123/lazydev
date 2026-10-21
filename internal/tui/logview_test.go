package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/claude"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// addEntries hands entries to lv as if read from the implement log.
func addEntries(lv *logView, entries ...claude.Entry) {
	from := lv.offsets[state.StepImplement]
	lv.apply(map[string]logChunk{state.StepImplement: {from: from, to: from + 1, entries: entries}})
}

func textEntry(s string) claude.Entry {
	return claude.Entry{Kind: claude.KindText, Text: s}
}

func TestLogViewWrap(t *testing.T) {
	lv := newLogView(1, "p", "checkout")
	lv.ctx.run = &state.Run{Branch: "feat/checkout", Base: "main", Phase: project.PhaseError}
	lv.Resize(40, 12)
	addEntries(lv,
		textEntry(strings.Repeat("word ", 20)+strings.Repeat("x", 70)),
		claude.Entry{Kind: claude.KindTool, Text: "Bash one\ntwo", Sub: true},
	)
	view := lv.View(true)
	checkSize(t, view, 40, 12)

	var body []string
	for _, l := range lv.body() {
		body = append(body, ansi.Strip(l))
	}
	if body[0] != "── implement ──" {
		t.Errorf("separator: %q", body[0])
	}
	// 100 characters of words and 70 of one word need more than 5 rows of 31 cells.
	if len(body) < 1+6+2 {
		t.Fatalf("body not wrapped: %q", body)
	}
	for _, l := range body[2:] {
		if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, strings.Repeat(" ", logStampWidth)) {
			t.Errorf("continuation line not indented: %q", l)
		}
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("line is %d wide: %q", w, l)
		}
	}
	last := body[len(body)-1]
	if last != strings.Repeat(" ", logStampWidth)+"  two" {
		t.Errorf("subagent continuation: %q", last)
	}
}

func TestLogViewFollow(t *testing.T) {
	lv := newLogView(1, "p", "checkout")
	lv.Resize(30, 6)
	for i := range 20 {
		addEntries(lv, textEntry(fmt.Sprintf("line %d", i)))
	}
	visible := func(s string) bool { return strings.Contains(ansi.Strip(lv.View(true)), s) }
	if !visible("line 19") {
		t.Fatal("view does not start at the end")
	}
	addEntries(lv, textEntry("line 20"))
	if !visible("line 20") {
		t.Error("view does not follow at the end")
	}
	lv.update("k")
	addEntries(lv, textEntry("line 21"))
	if visible("line 21") {
		t.Error("view follows after scrolling up")
	}
	lv.update("G")
	if !visible("line 21") {
		t.Error("view does not follow after G")
	}
	lv.update("g")
	if !visible("line 0") || visible("line 21") {
		t.Error("g does not go to the top")
	}
	checkSize(t, lv.View(true), 30, 6)
}

func TestLogViewReadsIncrementally(t *testing.T) {
	isolateTUI(t)
	write := func(step, data string) {
		path := state.LogPath("p", "checkout", step)
		os.MkdirAll(filepath.Dir(path), 0o755)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString(data)
		f.Close()
	}
	line := func(text string) string { return `{"type":"lazydev","text":"` + text + `"}` }
	lv := newLogView(1, "p", "checkout")
	lv.Resize(60, 20)
	read := func() { lv.apply(readSteps(lv.projectID, lv.prd, lv.offsets)) }

	write(state.StepSetup, line("setup done")+"\n")
	write(state.StepImplement, line("first")+"\n"+`{"type":"lazydev","text":"sec`)
	read()
	if got := len(lv.entries[state.StepImplement]); got != 1 {
		t.Fatalf("entries after the first read: %d", got)
	}
	write(state.StepImplement, `ond"}`+"\n"+line("third")+"\n")
	read()
	read()
	var texts []string
	for _, e := range lv.entries[state.StepImplement] {
		texts = append(texts, e.Text)
	}
	if strings.Join(texts, ",") != "first,second,third" {
		t.Errorf("implement entries: %q", texts)
	}
	if n := len(lv.entries[state.StepSetup]); n != 1 {
		t.Errorf("setup entries: %d", n)
	}
	body := ansi.Strip(strings.Join(lv.body(), "\n"))
	for _, sep := range []string{"── setup ──", "── implement ──"} {
		if strings.Count(body, sep) != 1 {
			t.Errorf("separator %q not once:\n%s", sep, body)
		}
	}
	if strings.Contains(body, "── review ──") {
		t.Errorf("separator of a step without log:\n%s", body)
	}
}

// logModel is a model with one project that has a run of checkout.
func logModel(t *testing.T) (model, *projectEntry, string) {
	t.Helper()
	isolateTUI(t)
	p := &projectEntry{path: t.TempDir(), runs: map[string]*state.Run{"checkout": {Phase: project.PhaseError}}}
	m := model{width: 160, height: 40, projects: []*projectEntry{p}}
	return m, p, projectIDs(m.projects)[0]
}

func countLogTabs(p *projectEntry) int {
	n := 0
	for _, tab := range p.tabs {
		if _, ok := tab.(*logView); ok {
			n++
		}
	}
	return n
}

func TestOpenLogFocusesExisting(t *testing.T) {
	m, p, id := logModel(t)
	m.openLog(id, "checkout")
	m.focus = colRight
	m.openLog(id, "checkout")
	if n := countLogTabs(p); n != 1 {
		t.Fatalf("log tabs: %d", n)
	}
	if m.focus != colCenter || p.activeLog() == nil || p.activeLog().TabTitle() != "log checkout" {
		t.Errorf("focus %d, active tab %v", m.focus, p.tabs[p.tab])
	}
	checkSize(t, m.View(), 160, 40)
}

func TestLogTabClosesOnMergeAndDiscard(t *testing.T) {
	for _, tc := range []struct {
		name string
		runs map[string]*state.Run
	}{
		{"merged", map[string]*state.Run{"checkout": {Phase: project.PhaseMerged}}},
		{"discarded", map[string]*state.Run{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, p, id := logModel(t)
			m.openLog(id, "checkout")
			m.Update(runsMsg{byID: map[string]map[string]*state.Run{id: {"checkout": {Phase: project.PhaseAborted}}}})
			if countLogTabs(p) != 1 {
				t.Fatal("an aborted run closed its log tab")
			}
			m.Update(runsMsg{byID: map[string]map[string]*state.Run{id: tc.runs}})
			if n := countLogTabs(p); n != 0 {
				t.Errorf("log tabs: %d", n)
			}
		})
	}
}
