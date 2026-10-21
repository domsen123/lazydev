package tui

import (
	"fmt"
	"sort"

	"lazydev/internal/project"
)

type prdSort int

const (
	sortByName prdSort = iota
	// sortByCreated zeigt die neuesten PRDs zuerst.
	sortByCreated
)

func (s prdSort) label() string {
	if s == sortByCreated {
		return "neueste zuerst"
	}
	return "Name"
}

// prdRow ist eine Zeile der PRD-Liste. Ohne prd ist sie die Gruppenzeile der erledigten PRDs.
type prdRow struct {
	prd  *project.PRD
	done int
	// cost ist die Summe der Kosten aller erledigten PRDs.
	cost float64
}

// prdList liefert die sichtbaren Zeilen: offene PRDs, die Gruppenzeile „erledigt“ und, falls
// aufgeklappt, die gemergten PRDs.
func (m Model) prdList(p *project.Project) []prdRow {
	if p == nil {
		return nil
	}
	var open, done []*project.PRD
	for i := range p.PRDs {
		d := &p.PRDs[i]
		if prdPhase(d, m.runs.get(p, d)) == project.PhaseMerged {
			done = append(done, d)
		} else {
			open = append(open, d)
		}
	}
	m.sortPRDs(open)
	m.sortPRDs(done)
	rows := make([]prdRow, 0, len(p.PRDs)+1)
	for _, d := range open {
		rows = append(rows, prdRow{prd: d})
	}
	if len(done) > 0 {
		group := prdRow{done: len(done)}
		for _, d := range done {
			if c, ok := prdCost(d, m.runs.get(p, d)); ok {
				group.cost += c
			}
		}
		rows = append(rows, group)
	}
	if m.showDone {
		for _, d := range done {
			rows = append(rows, prdRow{prd: d})
		}
	}
	return rows
}

func (m Model) sortPRDs(prds []*project.PRD) {
	sort.SliceStable(prds, func(i, j int) bool {
		a, b := prds[i], prds[j]
		if m.prdSort == sortByCreated && !a.Created.Equal(b.Created) {
			return a.Created.After(b.Created)
		}
		return a.Label() < b.Label()
	})
}

// groupLine ist der Text der Gruppenzeile.
func (m Model) groupLine(n int) string {
	arrow := "▸"
	if m.showDone {
		arrow = "▾"
	}
	return fmt.Sprintf("%s %d erledigt", arrow, n)
}

// selectPRD setzt den Cursor auf das PRD mit diesem Namen. Fehlt es in der Liste, bleibt der
// Cursor an seiner Stelle.
func (m *Model) selectPRD(name string) {
	rows := m.prdList(m.selectedProject())
	for i, r := range rows {
		if r.prd != nil && r.prd.Name == name {
			m.prdCursor = i
			return
		}
	}
	m.prdCursor = clamp(m.prdCursor, len(rows))
}

// toggleDone klappt die erledigten PRDs auf oder zu. Die Auswahl bleibt erhalten, außer das
// gewählte PRD verschwindet. Dann springt der Cursor auf die Gruppenzeile.
func (m *Model) toggleDone() {
	name := ""
	if d := m.selectedPRD(); d != nil {
		name = d.Name
	}
	m.showDone = !m.showDone
	rows := m.prdList(m.selectedProject())
	for i, r := range rows {
		if (name != "" && r.prd != nil && r.prd.Name == name) || (name == "" && r.prd == nil) {
			m.prdCursor = i
			return
		}
	}
	for i, r := range rows {
		if r.prd == nil {
			m.prdCursor = i
			return
		}
	}
	m.prdCursor = clamp(m.prdCursor, len(rows))
}

// toggleSort wechselt zwischen Name und Erstellzeit. Das gewählte PRD bleibt gewählt.
func (m *Model) toggleSort() {
	d := m.selectedPRD()
	m.prdSort = (m.prdSort + 1) % 2
	m.setMessage("Sortierung: "+m.prdSort.label(), false)
	if d != nil {
		m.selectPRD(d.Name)
	}
}
