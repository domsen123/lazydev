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

// TestCostAndModelFromPRD prüft Kosten und Modell aus der Statuszeile ohne Lauf-Zustand und den
// Vorrang des Lauf-Zustands, ohne dass ein PRD doppelt zählt.
func TestCostAndModelFromPRD(t *testing.T) {
	root := t.TempDir()
	dir := newTestProject(t, root, "alpha")
	for name, status := range map[string]string{
		"ohne-lauf": "umgesetzt (2026-10-01) · Kosten: 2.00 USD · Modell: claude-opus-5-5 · Scope: a",
		"mit-lauf":  "umgesetzt (2026-10-02) · Kosten: 1.00 USD · Modell: claude-sonnet-5-5",
	} {
		path := filepath.Join(dir, "prd", "auth", name+".md")
		if err := os.WriteFile(path, []byte("# PRD: "+name+"\n> Status: "+status+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &state.Run{PRD: "prd/auth/mit-lauf.md", Phase: project.PhaseMerged}
	cost := 1.5
	r.StartStep("implement")
	r.Steps[0].CostUSD, r.Steps[0].Model, r.Steps[0].OK = &cost, "claude-haiku-5-5", true
	if err := state.Save("alpha", "mit-lauf", r); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Projects: []config.Project{{Path: dir}}}
	m := drive(t, New(filepath.Join(root, "config.yaml"), cfg), tea.WindowSizeMsg{Width: 160, Height: 30})
	m = drive(t, m, m.Init()())
	m = drive(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m = drive(t, m, keys("e")...)

	view := ansi.Strip(m.View())
	for _, want := range []string{"Branch                 Modell     Kosten", "3.50 USD", "▾ 2 erledigt", "3.50 $", "Opus 5.5", "2.00 $", "Haiku 5.5", "1.50 $"} {
		if !strings.Contains(view, want) {
			t.Errorf("%q fehlt:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Sonnet") || strings.Contains(view, "4.50") {
		t.Errorf("Wert aus dem PRD trotz Lauf-Zustand:\n%s", view)
	}

	p := m.selectedProject()
	d, _ := p.FindPRD("ohne-lauf")
	meta := ansi.Strip(strings.Join(detailMeta(&d, nil), "\n"))
	if !strings.Contains(meta, "Kosten    2.00 USD") || !strings.Contains(meta, "Modell    claude-opus-5-5") {
		t.Errorf("detailMeta ohne Lauf:\n%s", meta)
	}
	d, _ = p.FindPRD("mit-lauf")
	meta = ansi.Strip(strings.Join(detailMeta(&d, r), "\n"))
	if !strings.Contains(meta, "Kosten    1.50 USD") || !strings.Contains(meta, "Modell    claude-haiku-5-5") {
		t.Errorf("detailMeta mit Lauf:\n%s", meta)
	}
	d, _ = p.FindPRD("token")
	if meta := ansi.Strip(strings.Join(detailMeta(&d, nil), "\n")); strings.Contains(meta, "Kosten") || strings.Contains(meta, "Modell") {
		t.Errorf("detailMeta ohne Werte:\n%s", meta)
	}
}

func TestShortModel(t *testing.T) {
	cases := map[string]string{
		"claude-opus-5-5":  "Opus 5.5",
		"claude-haiku-5-5": "Haiku 5.5",
		"claude-fable-5-1": "Fable 5.1",
		"claude-opus":      "Opus",
		"gpt-x-1":          "gpt-x-1",
		"":                 "",
	}
	for id, want := range cases {
		if got := shortModel(id); got != want {
			t.Errorf("shortModel(%q) = %q, erwartet %q", id, got, want)
		}
	}
	if got := padRight(shortModel("other-model-long"), colModel); got != "other-mo…" {
		t.Errorf("gekürzt: %q", got)
	}
}
