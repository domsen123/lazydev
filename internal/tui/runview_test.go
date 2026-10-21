package tui

import (
	"slices"
	"testing"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestRunList(t *testing.T) {
	p := &projectEntry{runs: map[string]*state.Run{
		"merged":  {Phase: project.PhaseMerged, StartedAt: "2026-10-21T08:00:00Z"},
		"late":    {Phase: project.PhaseError, StartedAt: "2026-10-21T12:00:00Z"},
		"early":   {Phase: project.PhaseAborted, StartedAt: "2026-10-21T09:00:00Z"},
		"b-tie":   {Phase: project.PhaseImplement, StartedAt: "2026-10-21T10:00:00Z"},
		"a-tie":   {Phase: project.PhaseReview, StartedAt: "2026-10-21T10:00:00Z"},
		"waiting": {Phase: project.PhaseWaiting, StartedAt: "2026-10-21T11:00:00Z"},
	}}
	want := []string{"early", "a-tie", "b-tie", "waiting", "late"}
	if got := p.runList(); !slices.Equal(got, want) {
		t.Errorf("runList() = %q, want %q", got, want)
	}

	// A cursor past the end clamps to the last run.
	p.runCursor = 10
	if got := p.selectedRun(); got != "late" {
		t.Errorf("selectedRun() = %q, want late", got)
	}
	if got := (&projectEntry{}).selectedRun(); got != "" {
		t.Errorf("selectedRun() without runs = %q", got)
	}
}
