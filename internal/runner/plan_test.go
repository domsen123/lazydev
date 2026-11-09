package runner

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

func TestParsePlan(t *testing.T) {
	known := []string{"alt", "schema"}
	for _, tc := range []struct {
		result, reason string
		want           []string
		err            bool
	}{
		{result: "x\n\n## Reihenfolge\n\nNach: keine\nGrund: getrennte Dateien\n", reason: "getrennte Dateien"},
		{result: "## Reihenfolge\n\nNach: `alt`, prd/db/schema.md, alt\nGrund: Migrationen\n", want: []string{"alt", "schema"}, reason: "Migrationen"},
		{result: "## Reihenfolge\n\n```\nNach: alt\nGrund: baut darauf auf\n```\n", want: []string{"alt"}, reason: "baut darauf auf"},
		{result: "## Reihenfolge\n\nNach: fremd\nGrund: x\n", err: true},
		{result: "Nach: alt\n", err: true},
		{result: "## Reihenfolge\n\nGrund: x\n", err: true},
	} {
		got, reason, err := parsePlan(tc.result, known)
		if (err != nil) != tc.err || !slices.Equal(got, tc.want) || (!tc.err && reason != tc.reason) {
			t.Errorf("parsePlan(%q) = %v, %q, %v", tc.result, got, reason, err)
		}
	}
}

// saveOtherRun stores the run of another PRD of the project in phase.
func saveOtherRun(t *testing.T, target Target, name string, phase project.Phase, after ...string) {
	t.Helper()
	r := &state.Run{ProjectDir: target.Dir, PRD: "prd/auth/" + name + ".md", Phase: phase, StartedAt: state.Now(), After: after}
	if err := state.Save(target.ID, name, r); err != nil {
		t.Fatal(err)
	}
}

// TestRunPlanWaitsForAfter checks that plan asks Claude read-only in the main folder, that the PRD
// waits for the run plan names without setup, and that it goes on once that run is merged.
func TestRunPlanWaitsForAfter(t *testing.T) {
	target := newProject(t, "", false)
	saveOtherRun(t, target, "alt", project.PhaseBlocked)
	t.Setenv("FAKE_PLAN", "alt")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseWaiting || r.PID != 0 || !slices.Equal(r.After, []string{"alt"}) || r.AfterReason != "beide ändern die Tokens" {
		t.Fatalf("state: %+v", r)
	}
	if r.NextStep() != state.StepSetup || len(r.Steps) != 1 || r.Steps[0].CostUSD == nil || *r.Steps[0].CostUSD != 0.05 {
		t.Errorf("steps: %+v", r.Steps)
	}
	args := fakeArgs(t)
	main, _ := filepath.EvalSymlinks(target.Dir)
	for _, s := range []string{"# lazydev plan: " + prdPath, "`alt` (Phase blocked)", "default", "Read,Glob,Grep", "PWD=" + main} {
		if !strings.Contains(args, s) {
			t.Errorf("claude args lack %q:\n%s", s, args)
		}
	}
	if log := read(t, state.LogPath("app", prdName, state.StepPlan)); !strings.Contains(log, "After alt: beide ändern die Tokens") {
		t.Errorf("plan log:\n%s", log)
	}
	if waiting, started := waitsForAfter("app", r), r.NextStep(); !waiting || started != state.StepSetup {
		t.Errorf("waitsForAfter = %v", waiting)
	}

	saveOtherRun(t, target, "alt", project.PhaseMerged)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged || len(r.Steps) != 5 {
		t.Errorf("state after alt merged: %+v", r)
	}
}

// TestRunPlanParallel checks that "Nach: keine" goes on with the setup right away.
func TestRunPlanParallel(t *testing.T) {
	target := newProject(t, "", false)
	saveOtherRun(t, target, "alt", project.PhaseWaiting)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged || len(r.After) != 0 || r.AfterReason != "beide ändern die Tokens" {
		t.Errorf("state: %+v", r)
	}
}

// TestPendingAfter checks that a merged or discarded dependency no longer holds a PRD back.
func TestPendingAfter(t *testing.T) {
	target := newProject(t, "", false)
	saveOtherRun(t, target, "merged", project.PhaseMerged)
	saveOtherRun(t, target, "blocked", project.PhaseBlocked)
	r := &state.Run{After: []string{"merged", "blocked", "discarded"}}
	if got := PendingAfter(target.ID, r); !slices.Equal(got, []string{"blocked"}) {
		t.Errorf("PendingAfter = %v", got)
	}
}

func TestPlanDropsCycles(t *testing.T) {
	target := newProject(t, "", false)
	saveOtherRun(t, target, "a", project.PhaseWaiting, "b")
	saveOtherRun(t, target, "b", project.PhaseWaiting, prdName)
	after := []string{"a", "c"}
	if dropped := dropCycles(target.ID, prdName, &after); !slices.Equal(dropped, []string{"a"}) || !slices.Equal(after, []string{"c"}) {
		t.Errorf("dropCycles = %v, after %v", dropped, after)
	}
}

func TestSetAfter(t *testing.T) {
	target := newProject(t, "", false)
	saveOtherRun(t, target, "alt", project.PhaseImplement, prdName)
	saveOtherRun(t, target, "schema", project.PhaseBlocked)
	if err := SetAfter(target, prdName, []string{"schema"}); err == nil || !strings.Contains(err.Error(), "has no run") {
		t.Fatalf("SetAfter without run: %v", err)
	}
	saveOtherRun(t, target, prdName, project.PhaseWaiting)
	if err := SetAfter(target, prdName, []string{"alt"}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("SetAfter with a cycle: %v", err)
	}
	if err := SetAfter(target, prdName, []string{prdName}); err == nil {
		t.Error("SetAfter on itself works")
	}
	if err := SetAfter(target, prdName, []string{"prd/auth/schema.md"}); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if !slices.Equal(r.After, []string{"schema"}) || r.AfterReason != "set by hand" || r.NextStep() != state.StepSetup || r.Phase != project.PhaseWaiting {
		t.Errorf("state: %+v", r)
	}
	// Clearing After makes it the turn of the PRD: startWaiting tries to spawn it, which fails in tests.
	if err := SetAfter(target, prdName, nil); err == nil || !strings.Contains(err.Error(), "no runner spawns in tests") {
		t.Errorf("SetAfter clearing: %v", err)
	}
	if r := loadRun(t); len(r.After) != 0 || waitsForAfter(target.ID, r) {
		t.Errorf("After not cleared: %+v", r)
	}
}
