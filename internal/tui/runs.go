package tui

import (
	"path/filepath"
	"sort"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

// runKey verbindet einen Lauf mit seinem Projekt und PRD.
type runKey struct{ projectID, prdName string }

type runs map[runKey]*state.Run

func loadRuns() runs {
	entries, _ := state.All()
	rs := runs{}
	for _, e := range entries {
		rs[runKey{e.ProjectID, e.PRDName}] = e.Run
	}
	return rs
}

func (rs runs) get(p *project.Project, d *project.PRD) *state.Run {
	if p == nil || d == nil {
		return nil
	}
	return rs[runKey{p.ID, d.Name}]
}

// signature fasst zusammen, was eine neue PRD-Liste nötig macht: Phasen und Worktrees. Ändert
// sich nur ein Log oder eine Kostenangabe, bleibt die Signatur gleich.
func (rs runs) signature() map[runKey]project.Phase {
	sig := map[runKey]project.Phase{}
	for k, r := range rs {
		sig[k] = r.Phase
	}
	return sig
}

func sameSignature(a, b map[runKey]project.Phase) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// addWorktreePRDs ergänzt PRDs, die ein Lauf aus dem Hauptordner in seinen Worktree verschoben
// hat. Ohne sie fehlte jedes laufende PRD in der Liste.
func addWorktreePRDs(p *project.Project, rs runs) {
	known := map[string]bool{}
	for _, d := range p.PRDs {
		known[d.Name] = true
	}
	added := false
	for k, r := range rs {
		if k.projectID != p.ID || known[k.prdName] || r.PRD == "" || r.Phase == project.PhaseMerged {
			continue
		}
		d, err := project.ReadPRD(r.Worktree, filepath.Join(r.Worktree, r.PRD))
		if err != nil {
			// Ohne Worktree-Datei zeigt die Liste das PRD trotzdem, damit sich der Lauf verwerfen lässt.
			d = project.PRD{Path: r.PRD, Name: k.prdName, Area: filepath.Base(filepath.Dir(r.PRD))}
		}
		d.Tracked = true
		d.Branch = r.Branch
		p.PRDs = append(p.PRDs, d)
		added = true
	}
	if added {
		sort.Slice(p.PRDs, func(i, j int) bool { return p.PRDs[i].Path < p.PRDs[j].Path })
	}
}

// prdPhase nimmt die Phase aus dem Lauf, sonst aus der Statuszeile.
func prdPhase(d *project.PRD, r *state.Run) project.Phase {
	if r != nil {
		return r.Effective()
	}
	return d.Phase()
}

// prdCost nimmt die Kosten aus dem Lauf, wenn er welche hat, sonst aus der Statuszeile.
func prdCost(d *project.PRD, r *state.Run) (float64, bool) {
	if r != nil && r.Cost() > 0 {
		return r.Cost(), true
	}
	return d.CostUSD, d.HasCost
}

// prdModel nimmt das Modell des letzten erfolgreichen implement aus dem Lauf, sonst aus der
// Statuszeile.
func prdModel(d *project.PRD, r *state.Run) string {
	if r != nil {
		if m := r.ImplementModel(); m != "" {
			return m
		}
	}
	return d.Model
}

// projectCost summiert die Kosten aller PRDs eines Projekts nach prdCost.
func (rs runs) projectCost(p *project.Project) float64 {
	total := 0.0
	for i := range p.PRDs {
		d := &p.PRDs[i]
		if c, ok := prdCost(d, rs.get(p, d)); ok {
			total += c
		}
	}
	return total
}

// activeRuns zählt die laufenden Runner eines Projekts.
func (rs runs) activeRuns(projectID string) int {
	n := 0
	for k, r := range rs {
		if k.projectID == projectID && r.Running() {
			n++
		}
	}
	return n
}
