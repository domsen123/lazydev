package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestDoneGroupAndSort(t *testing.T) {
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	for name, status := range map[string]string{"zeta": "geplant", "erledigt": "umgesetzt (2026-10-01)"} {
		path := filepath.Join(dir, "prd", "auth", name+".md")
		if err := os.WriteFile(path, []byte("# PRD: "+name+"\n> Status: "+status+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &state.Run{PRD: "prd/auth/erledigt.md", Phase: project.PhaseMerged}
	cost := 1.5
	r.StartStep("implement")
	r.Steps[0].CostUSD = &cost
	if err := state.Save("alpha", "erledigt", r); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Projects: []config.Project{{Path: dir}}}
	m := drive(t, New(filepath.Join(root, "config.yaml"), cfg), tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(t, m, m.Init()())
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})

	labels := func() []string {
		var out []string
		for _, r := range m.prdList(m.selectedProject()) {
			if r.prd == nil {
				out = append(out, "erledigt")
			} else {
				out = append(out, r.prd.Name)
			}
		}
		return out
	}
	if got := strings.Join(labels(), " "); got != "token zeta erledigt" {
		t.Errorf("zugeklappt: %s", got)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "▸ 1 erledigt") || !strings.Contains(view, "1.50 $") || strings.Contains(view, "auth/erledigt") {
		t.Errorf("Gruppenzeile fehlt oder erledigtes PRD sichtbar:\n%s", view)
	}

	// Enter auf der Gruppenzeile klappt auf, die Auswahl bleibt auf der Gruppenzeile.
	m = drive(t, m, keys("jj")...)
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := strings.Join(labels(), " "); got != "token zeta erledigt erledigt" || m.selectedPRD() != nil {
		t.Errorf("aufgeklappt: %s, Auswahl %v", got, m.selectedPRD())
	}

	// Sortieren nach Erstellzeit, neueste zuerst. Die Auswahl folgt dem PRD.
	p := m.selectedProject()
	for i := range p.PRDs {
		if p.PRDs[i].Name == "token" {
			p.PRDs[i].Created = time.Now()
		} else {
			p.PRDs[i].Created = time.Now().Add(-time.Hour)
		}
	}
	m = drive(t, m, keys("k")...)
	m = drive(t, m, keys("S")...)
	if got := strings.Join(labels(), " "); got != "token zeta erledigt erledigt" {
		t.Errorf("neueste zuerst: %s", got)
	}
	for i := range p.PRDs {
		if p.PRDs[i].Name == "zeta" {
			p.PRDs[i].Created = time.Now().Add(time.Hour)
		}
	}
	if got := strings.Join(labels(), " "); got != "zeta token erledigt erledigt" {
		t.Errorf("neueste zuerst nach Änderung: %s", got)
	}

	// e klappt wieder zu. Das gewählte erledigte PRD verschwindet, der Cursor geht auf die Gruppe.
	m = drive(t, m, keys("jjj")...)
	if d := m.selectedPRD(); d == nil || d.Name != "erledigt" {
		t.Fatalf("Auswahl: %v", d)
	}
	m = drive(t, m, keys("e")...)
	if m.selectedPRD() != nil || m.prdCursor != 2 {
		t.Errorf("Cursor nach Zuklappen: %d", m.prdCursor)
	}
}
