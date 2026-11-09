package runner

import (
	"testing"

	"lazydev/internal/project"
)

func TestChanges(t *testing.T) {
	before := []Item{
		{Project: "p", PRD: "a", Phase: project.PhaseImplement},
		{Project: "p", PRD: "b", Phase: project.PhaseMergeQueue},
		{Project: "p", PRD: "c", Phase: project.PhaseMerged},
		{Project: "p", PRD: "d", Phase: project.PhaseReview},
	}
	after := []Item{
		{Project: "p", PRD: "a", Phase: project.PhaseBlocked, Blocker: &Blocker{Step: "implement", Questions: []string{"Q1", "Q2"}}},
		{Project: "p", PRD: "b", Phase: project.PhaseMergeQueue, Note: "main folder has uncommitted changes"},
		{Project: "p", PRD: "c", Phase: project.PhaseMerged},
		{Project: "p", PRD: "d", Phase: project.PhaseMerge},
		{Project: "p", PRD: "e", Phase: project.PhaseMerged},
	}
	got := Changes(before, after)
	if len(got) != 2 {
		t.Fatalf("Changes = %+v, want 2 events", got)
	}
	if e := got[0]; e.Kind != "blocked" || e.PRD != "a" || e.From != project.PhaseImplement || e.Text != "Q1 | Q2" {
		t.Errorf("event 0 = %+v", e)
	}
	if e := got[1]; e.Kind != "note" || e.PRD != "b" || e.Text != "main folder has uncommitted changes" {
		t.Errorf("event 1 = %+v", e)
	}
}

func TestBusy(t *testing.T) {
	if Busy([]Item{{Phase: project.PhaseMerged}, {Phase: project.PhaseBlocked}}) {
		t.Error("Busy with only merged and blocked")
	}
	for _, ph := range []project.Phase{project.PhaseWaiting, project.PhaseMergeQueue, project.PhaseImplement} {
		if !Busy([]Item{{Phase: ph}}) {
			t.Errorf("not Busy with %s", ph)
		}
	}
}
