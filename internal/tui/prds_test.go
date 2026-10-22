package tui

import (
	"slices"
	"testing"
	"time"

	"lazydev/internal/project"
)

func TestPRDRowsNewestFirst(t *testing.T) {
	now := time.Now()
	prd := func(name string, age time.Duration, kind project.StatusKind) project.PRD {
		return project.PRD{Path: "prd/" + name + ".md", Name: name, ModTime: now.Add(-age), Status: project.Status{Kind: kind}}
	}
	p := &projectEntry{showDone: true, prds: []project.PRD{
		prd("a", time.Hour, project.StatusPlanned),
		prd("b", time.Minute, project.StatusPlanned),
		prd("c", time.Hour, project.StatusPlanned),
		prd("d", 2*time.Hour, project.StatusImplemented),
		prd("e", time.Minute, project.StatusImplemented),
	}}
	var got []string
	for _, r := range p.prdRows() {
		if r.prd == nil {
			got = append(got, "-")
		} else {
			got = append(got, r.prd.Name)
		}
	}
	if want := []string{"b", "a", "c", "-", "e", "d"}; !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}
