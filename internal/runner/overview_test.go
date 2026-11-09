package runner

import (
	"strings"
	"testing"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestChanges(t *testing.T) {
	before := []Item{
		{Project: "p", PRD: "a", Phase: project.PhaseImplement},
		{Project: "p", PRD: "b", Phase: project.PhaseMergeQueue},
		{Project: "p", PRD: "c", Phase: project.PhaseMerged},
		{Project: "p", PRD: "d", Phase: project.PhaseReview},
		{Project: "p", PRD: "f", Phase: project.PhaseResolve},
	}
	after := []Item{
		{Project: "p", PRD: "a", Phase: project.PhaseBlocked, Blocker: &Blocker{Step: "implement", Questions: []string{"Q1", "Q2"}}},
		{Project: "p", PRD: "b", Phase: project.PhaseMergeQueue, Note: "main folder has uncommitted changes"},
		{Project: "p", PRD: "c", Phase: project.PhaseMerged},
		{Project: "p", PRD: "d", Phase: project.PhaseMerge},
		{Project: "p", PRD: "e", Phase: project.PhaseMerged},
		{Project: "p", PRD: "f", Phase: project.PhaseImplement, Resolved: 1, File: "/wt/f.md"},
	}
	got := Changes(before, after)
	if len(got) != 3 {
		t.Fatalf("Changes = %+v, want 2 events", got)
	}
	if e := got[0]; e.Kind != "blocked" || e.PRD != "a" || e.From != project.PhaseImplement || e.Text != "Q1 | Q2" {
		t.Errorf("event 0 = %+v", e)
	}
	if e := got[1]; e.Kind != "note" || e.PRD != "b" || e.Text != "main folder has uncommitted changes" {
		t.Errorf("event 1 = %+v", e)
	}
	if e := got[2]; e.Kind != "resolved" || e.PRD != "f" || !strings.Contains(e.Text, "/wt/f.md") {
		t.Errorf("event 2 = %+v", e)
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

func TestStepInfos(t *testing.T) {
	ok, failed := 0, 1
	cost := 0.5
	r := &state.Run{Steps: []state.Step{
		{Name: state.StepPlan, ExitCode: &ok, OK: true},
		{Name: state.StepSetup, ExitCode: &ok, OK: true},
		{Name: state.StepImplement, ExitCode: &failed, OK: false},
		{Name: state.StepResolve, ExitCode: &ok, OK: true},
		{Name: state.StepImplement, ExitCode: &ok, OK: true, CostUSD: &cost},
		{Name: state.StepReview},
	}}
	var got []string
	for _, s := range stepInfos(r) {
		got = append(got, s.Name+":"+s.State)
	}
	// The runner of r does not live, so the unfinished review counts as failed.
	want := "plan:done setup:done implement:done review:failed merge:pending"
	if strings.Join(got, " ") != want {
		t.Errorf("stepInfos = %v, want %s", got, want)
	}
	if stepInfos(r)[2].Cost != 0.5 {
		t.Error("implement cost missing")
	}
}
