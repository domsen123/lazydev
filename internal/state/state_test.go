package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"lazydev/internal/project"
)

func TestSaveLoadRemove(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if _, err := Load("p", "x"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load ohne Datei: %v", err)
	}
	r := &Run{PRD: "prd/a/x.md", Port: 3101, Phase: project.PhaseSetup}
	r.StartStep("setup")
	r.EndStep(0, true)
	if err := Save("p", "x", r); err != nil {
		t.Fatal(err)
	}
	if err := Save("q", "y", &Run{Port: 3102}); err != nil {
		t.Fatal(err)
	}
	got, err := Load("p", "x")
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 3101 || len(got.Steps) != 1 || *got.Steps[0].ExitCode != 0 || got.Steps[0].EndedAt == "" {
		t.Errorf("geladen: %+v", got)
	}

	ports, err := UsedPorts()
	if err != nil || !ports[3101] || !ports[3102] {
		t.Errorf("UsedPorts = %v, %v", ports, err)
	}

	// Logs dieses PRDs und eines PRDs mit Punkt im Namen anlegen.
	for _, f := range []string{"x.setup.jsonl", "x.implement.jsonl", "x.v2.json", "x.v2.setup.jsonl"} {
		os.WriteFile(filepath.Join(RunsDir("p"), f), nil, 0o644)
	}
	if err := Remove("p", "x"); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(RunsDir("p"), "*"))
	if len(left) != 2 {
		t.Errorf("nach Remove übrig: %v", left)
	}
}

func TestFail(t *testing.T) {
	r := &Run{Phase: project.PhaseSetup}
	r.Fail(errors.New("kaputt"))
	if r.Phase != project.PhaseError || r.Error == nil || *r.Error != "kaputt" {
		t.Errorf("Fail: %+v", r)
	}
}

func TestAllAndEffective(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	Save("p", "b", &Run{StartedAt: "2026-10-20T10:00:00+02:00", Phase: project.PhaseImplement, PID: os.Getpid()})
	Save("p", "a", &Run{StartedAt: "2026-10-20T11:00:00+02:00", Phase: project.PhaseWaiting})
	entries, err := All()
	if err != nil || len(entries) != 2 || entries[0].PRDName != "b" || entries[1].ProjectID != "p" {
		t.Fatalf("All = %+v, %v", entries, err)
	}
	// Der Testprozess ist kein "lazydev run", der Lauf gilt also als abgebrochen.
	if got := entries[0].Run.Effective(); got != project.PhaseAborted {
		t.Errorf("Effective = %s", got)
	}
	if got := entries[1].Run.Effective(); got != project.PhaseWaiting {
		t.Errorf("Effective wartend = %s", got)
	}
}

func TestLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	unlock, err := Lock("x")
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	unlock, err = Lock("x")
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
