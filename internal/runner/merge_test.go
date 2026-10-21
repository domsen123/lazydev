package runner

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
	"lazydev/internal/worktree"
)

// enableAutoMerge liefert das Ziel neu, weil Target die Projekt-Konfiguration als Kopie hält.
func enableAutoMerge(t *testing.T, cfg *config.Config) Target {
	t.Helper()
	on := true
	cfg.Projects[0].AutoMerge = &on
	target, err := FindProject(cfg, "app")
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestMergeFastForward(t *testing.T) {
	cfg, target := setupProject(t)
	target = enableAutoMerge(t, cfg)
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	r, _ := state.Load("app", "new-token")
	if r.Phase != project.PhaseMerged || len(r.Steps) != 4 || !r.Steps[3].OK || r.Note != "" {
		t.Errorf("Zustand nach Merge: %+v", r)
	}
	main := target.Dir()
	if got := runGit(t, main, "log", "--format=%s", "-3"); got != "feat: review\ndocs(prd): add new-token\ninit" {
		t.Errorf("main: %q", got)
	}
	if runGit(t, main, "rev-list", "--merges", "--count", "HEAD") != "0" {
		t.Error("Merge-Commit entstanden")
	}
	if _, err := os.Stat(worktree.Path("app", "new-token")); err == nil {
		t.Error("Worktree existiert noch")
	}
	if runGit(t, main, "branch", "--list", "feat/new-token") != "" {
		t.Error("Branch existiert noch")
	}
	if ports, _ := state.UsedPorts(); ports[r.Port] {
		t.Error("Port eines gemergten Laufs bleibt belegt")
	}
	if err := Run(cfg, target, "new-token", io.Discard); err == nil || !strings.Contains(err.Error(), "nichts mehr") {
		t.Errorf("Run nach Merge: %v", err)
	}
}

func TestMergeNeedsApprovalWithoutAutoMerge(t *testing.T) {
	cfg, target := setupProject(t)
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	var spawned bool
	spawnRunner = func(string, string) (int, error) { spawned = true; return 0, nil }
	if _, err := Merge(cfg, target, "tracked"); err == nil {
		t.Error("Merge ohne Lauf akzeptiert")
	}
	if _, err := Merge(cfg, target, "new-token"); err != nil || !spawned {
		t.Fatalf("Merge: %v, gestartet %v", err, spawned)
	}
	// Der gestartete Runner läuft hier im Vordergrund.
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	if r, _ := state.Load("app", "new-token"); r.Phase != project.PhaseMerged {
		t.Errorf("Phase nach Freigabe: %s", r.Phase)
	}
}

// diverge erzeugt nach dem Review einen Commit in main, der review.txt anders anlegt als der Branch.
func diverge(t *testing.T, cfg *config.Config, target Target) Target {
	t.Helper()
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	write(t, target.Dir(), "review.txt", "main\n")
	runGit(t, target.Dir(), "add", "review.txt")
	runGit(t, target.Dir(), "commit", "-q", "-m", "main: review.txt")
	return enableAutoMerge(t, cfg)
}

func TestMergeConflictUsesMergePRD(t *testing.T) {
	cfg, target := setupProject(t)
	target = diverge(t, cfg, target)
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	main := target.Dir()
	if got := runGit(t, main, "log", "--format=%s", "-4"); got != "feat: review\ndocs(prd): add new-token\nmain: review.txt\ninit" {
		t.Errorf("main: %q", got)
	}
	r, _ := state.Load("app", "new-token")
	if s := r.Steps[3]; s.SessionID != "sess-1" || s.CostUSD == nil {
		t.Errorf("Sitzung von /merge-prd fehlt: %+v", s)
	}
}

func TestMergeConflictUnresolved(t *testing.T) {
	cfg, target := setupProject(t)
	target = diverge(t, cfg, target)
	t.Setenv("FAKE_MERGE_MODE", "abort")
	before := runGit(t, target.Dir(), "rev-parse", "main")
	err := Run(cfg, target, "new-token", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "beide Seiten unklar") {
		t.Fatalf("Fehler: %v", err)
	}
	r, _ := state.Load("app", "new-token")
	if r.Phase != project.PhaseError || NextStep(r) != StepMerge {
		t.Errorf("Zustand: %+v", r)
	}
	if runGit(t, target.Dir(), "rev-parse", "main") != before {
		t.Error("main verändert")
	}
	if _, err := os.Stat(worktree.Path("app", "new-token")); err != nil {
		t.Error("Worktree nach Fehler entfernt")
	}
}

func TestMergeCheckFailureUsesMergePRD(t *testing.T) {
	cfg, target := setupProject(t)
	write(t, target.Dir(), "check.sh", "#!/bin/sh\ntest -f .merge-fixed || { echo rot; exit 1; }\n")
	os.Chmod(filepath.Join(target.Dir(), "check.sh"), 0o755)
	runGit(t, target.Dir(), "add", "check.sh")
	runGit(t, target.Dir(), "commit", "-q", "-m", "check")
	cfg.Projects[0].Hooks.Check = "./check.sh"
	target = enableAutoMerge(t, cfg)
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	log, _ := os.ReadFile(state.LogPath("app", "new-token", StepMerge))
	if !strings.Contains(string(log), "Prüfung rot") || !strings.Contains(string(log), "/merge-prd feat/new-token") {
		t.Errorf("Log:\n%s", log)
	}
}

func TestMergeWaitsForCleanMain(t *testing.T) {
	cfg, target := setupProject(t)
	target = enableAutoMerge(t, cfg)
	write(t, target.Dir(), "setup.sh", "#!/bin/sh\necho geändert >> setup.sh\n")

	done := make(chan error, 1)
	go func() { done <- Run(cfg, target, "new-token", io.Discard) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if r, err := state.Load("app", "new-token"); err == nil && strings.HasPrefix(r.Note, NoteMainDirty) {
			if r.Phase != project.PhaseMergeQueue {
				t.Errorf("Phase beim Warten: %s", r.Phase)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Merge wartet nicht auf den Hauptordner")
		}
		time.Sleep(10 * time.Millisecond)
	}
	runGit(t, target.Dir(), "checkout", "--", "setup.sh")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if r, _ := state.Load("app", "new-token"); r.Phase != project.PhaseMerged || r.Note != "" {
		t.Errorf("Zustand: %+v", r)
	}
}

func TestMainDirProblem(t *testing.T) {
	_, target := setupProject(t)
	dir := target.Dir()
	if p, err := mainDirProblem(dir, "main", nil); err != nil || p != "" {
		t.Errorf("ungetracktes PRD blockiert: %q, %v", p, err)
	}
	write(t, dir, "setup.sh", "x")
	if p, _ := mainDirProblem(dir, "main", []string{"setup.sh"}); p != "" {
		t.Errorf("ignorierte Datei blockiert: %q", p)
	}
	write(t, dir, "neu.txt", "x")
	if p, _ := mainDirProblem(dir, "main", []string{"setup.sh"}); !strings.Contains(p, "neu.txt") {
		t.Errorf("ungetrackte Datei: %q", p)
	}
	runGit(t, dir, "switch", "-q", "-c", "other")
	if p, _ := mainDirProblem(dir, "main", []string{"setup.sh"}); !strings.Contains(p, "Branch other") {
		t.Errorf("anderer Branch: %q", p)
	}
}

// TestMergeIntoBaseBranch prüft einen Ziel-Branch außer main: Der Worktree zweigt von next ab,
// der Fast-Forward geht nach next, main bleibt unverändert.
func TestMergeIntoBaseBranch(t *testing.T) {
	cfg, _ := setupProject(t)
	main := cfg.Projects[0].AbsPath()
	runGit(t, main, "switch", "-q", "-c", "next")
	write(t, main, "next.txt", "nur auf next\n")
	runGit(t, main, "add", "next.txt")
	runGit(t, main, "commit", "-q", "-m", "next only")
	mainHead := runGit(t, main, "rev-parse", "main")
	cfg.Projects[0].BaseBranch = "next"
	target := enableAutoMerge(t, cfg)

	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	r, _ := state.Load("app", "new-token")
	if r.Phase != project.PhaseMerged || r.Base != "next" {
		t.Errorf("Zustand nach Merge: %+v", r)
	}
	if got := runGit(t, main, "log", "--format=%s", "-4", "next"); got != "feat: review\ndocs(prd): add new-token\nnext only\ninit" {
		t.Errorf("next: %q", got)
	}
	if got := runGit(t, main, "rev-parse", "main"); got != mainHead {
		t.Error("main hat sich verändert")
	}
}

func TestMainDirProblemUsesBase(t *testing.T) {
	cfg, _ := setupProject(t)
	dir := cfg.Projects[0].AbsPath()
	if p, _ := mainDirProblem(dir, "next", nil); !strings.Contains(p, "Branch main statt next") {
		t.Errorf("Problem: %q", p)
	}
}
