package runner

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
	"lazydev/internal/worktree"
)

// setupProject legt ein Git-Repo mit einem getrackten und einem ungetrackten PRD an und
// trägt es mit einem Setup-Skript statt eines Presets ein.
func setupProject(t *testing.T) (*config.Config, Target) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")

	dir := filepath.Join(t.TempDir(), "app")
	write(t, dir, "prd/auth/tracked.md", "# PRD: Tracked\n\n> Status: geplant · Scope: A\n")
	write(t, dir, ".gitignore", "port.txt\n.fake-args\n.merge-fixed\n")
	write(t, dir, "setup.sh", "#!/bin/sh\necho \"$LAZYDEV_PORT\" > port.txt\n")
	os.Chmod(filepath.Join(dir, "setup.sh"), 0o755)
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	write(t, dir, "prd/auth/new-token.md", "# PRD: Neu\n\n> Status: geplant · Scope: B\n")

	cfg, _ := config.Load(filepath.Join(t.TempDir(), "none.yaml"))
	cfg.PortBase = 41000
	cfg.Claude.Binary = fakeClaude(t)
	t.Setenv("FAKE_CLAUDE_MODE", "ok")
	t.Setenv("FAKE_MERGE_MODE", "theirs")
	mergePollInterval = 10 * time.Millisecond
	// Kein Test startet einen echten Runner im Hintergrund.
	spawnRunner = func(string, string) (int, error) { return 0, errors.New("spawnRunner im Test") }
	// Ohne autoMerge endet ein Lauf vor dem Merge. Die Merge-Tests schalten ihn ein.
	autoMerge := false
	cfg.Projects = []config.Project{{Path: dir, AutoMerge: &autoMerge, Hooks: config.Hooks{Setup: "./setup.sh"}}}
	target, err := FindProject(cfg, "app")
	if err != nil {
		t.Fatal(err)
	}
	return cfg, target
}

func TestRunMovesUntrackedPRD(t *testing.T) {
	cfg, target := setupProject(t)
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	r, err := state.Load("app", "new-token")
	if err != nil {
		t.Fatal(err)
	}
	wt := worktree.Path("app", "new-token")
	if r.Worktree != wt || r.Branch != "feat/new-token" || r.Slug != "new_token" || r.Port < 41000 {
		t.Errorf("Zustand: %+v", r)
	}
	if r.Phase != project.PhaseMergeQueue || r.Error != nil || r.PID != 0 || len(r.Steps) != 3 || !strings.Contains(r.Note, "lazydev merge") {
		t.Errorf("Phase oder Schritte: %+v", r)
	}
	for i, name := range []string{StepSetup, StepImplement, StepReview} {
		if s := r.Steps[i]; s.Name != name || *s.ExitCode != 0 || !s.OK {
			t.Errorf("Schritt %d: %+v", i, s)
		}
	}
	if s := r.Steps[1]; s.SessionID != "sess-1" || s.Model != "claude-opus-5-5" || s.CostUSD == nil || *s.CostUSD != 0.5 {
		t.Errorf("Sitzung, Modell oder Kosten fehlen: %+v", s)
	}
	if got := r.ImplementModel(); got != "claude-opus-5-5" {
		t.Errorf("ImplementModel = %q", got)
	}

	if _, err := os.Stat(filepath.Join(target.Dir(), "prd/auth/new-token.md")); err == nil {
		t.Error("PRD liegt noch im Hauptordner")
	}
	if got := runGit(t, wt, "log", "--format=%s", "main.."); got != "feat: review\ndocs(prd): add new-token" {
		t.Errorf("Commits im Worktree: %q", got)
	}
	if out := runGit(t, target.Dir(), "status", "--porcelain"); out != "" {
		t.Errorf("Hauptordner nicht sauber: %q", out)
	}
	port, _ := os.ReadFile(filepath.Join(wt, "port.txt"))
	if strings.TrimSpace(string(port)) == "" {
		t.Error("Setup-Skript lief nicht im Worktree")
	}
	logData, _ := os.ReadFile(state.LogPath("app", "new-token", "setup"))
	if !strings.Contains(string(logData), "Setup fertig") {
		t.Errorf("Log:\n%s", logData)
	}

	implLog, _ := os.ReadFile(state.LogPath("app", "new-token", StepImplement))
	if !strings.Contains(string(implLog), `"type":"result"`) || !strings.Contains(string(implLog), `"type":"stderr"`) {
		t.Errorf("Log von implement:\n%s", implLog)
	}
	if !strings.Contains(string(implLog), "Prompt: Standard") || strings.Contains(string(implLog), "Knowledge:") {
		t.Errorf("Prompt-Quelle fehlt im Log:\n%s", implLog)
	}
	args, _ := os.ReadFile(filepath.Join(wt, ".fake-args"))
	if !strings.Contains(string(args), "# lazydev implement: prd/auth/new-token.md\n") ||
		!strings.Contains(string(args), "## Regeln von lazydev") ||
		!strings.Contains(string(args), "Bash(git status:*),") || !strings.Contains(string(args), "GIT_EDITOR=true") ||
		!strings.Contains(string(args), "--agents\n"+claude.AgentsPath()) {
		t.Errorf("Aufruf von claude:\n%s", args)
	}

	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Errorf("zweiter Run ohne Freigabe: %v", err)
	}
	if r, _ := state.Load("app", "new-token"); len(r.Steps) != 3 || r.Phase != project.PhaseMergeQueue {
		t.Errorf("zweiter Run ohne Freigabe hat etwas ausgeführt: %+v", r)
	}
}

func TestRunTrackedPRDAndPortAllocation(t *testing.T) {
	cfg, target := setupProject(t)
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := Run(cfg, target, "tracked", io.Discard); err != nil {
		t.Fatal(err)
	}
	a, _ := state.Load("app", "new-token")
	b, _ := state.Load("app", "tracked")
	if a.Port == b.Port {
		t.Errorf("beide Läufe auf Port %d", a.Port)
	}
	wt := worktree.Path("app", "tracked")
	if got := runGit(t, wt, "log", "--format=%s", "main.."); got != "feat: review" {
		t.Errorf("getracktes PRD bekam einen Commit: %q", got)
	}
}

func TestRunFailureKeepsWorktree(t *testing.T) {
	cfg, target := setupProject(t)
	write(t, target.Dir(), "setup.sh", "#!/bin/sh\necho kaputt >&2\nexit 1\n")
	err := Run(cfg, target, "new-token", io.Discard)
	if err == nil {
		t.Fatal("Fehler erwartet")
	}
	r, _ := state.Load("app", "new-token")
	if r.Phase != project.PhaseError || r.Error == nil || *r.Steps[0].ExitCode != 1 || len(r.Steps) != 1 {
		t.Errorf("Zustand nach Fehler: %+v", r)
	}
	if _, err := os.Stat(worktree.Path("app", "new-token")); err != nil {
		t.Error("Worktree nach Fehler entfernt")
	}
	logData, _ := os.ReadFile(state.LogPath("app", "new-token", "setup"))
	if !strings.Contains(string(logData), "kaputt") {
		t.Errorf("stderr fehlt im Log:\n%s", logData)
	}
}

func TestTeardownRestoresPRD(t *testing.T) {
	cfg, target := setupProject(t)
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	wt := worktree.Path("app", "new-token")

	if err := Teardown(target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(target.Dir(), "prd/auth/new-token.md"))
	if err != nil {
		t.Fatal("PRD nicht zurück im Hauptordner")
	}
	if !strings.Contains(string(restored), "> Status: geplant · Scope: B") {
		t.Errorf("Status nicht zurückgesetzt:\n%s", restored)
	}
	if _, err := os.Stat(wt); err == nil {
		t.Error("Worktree existiert noch")
	}
	if out := runGit(t, target.Dir(), "branch", "--list", "feat/new-token"); out != "" {
		t.Error("Branch existiert noch")
	}
	if _, err := state.Load("app", "new-token"); err == nil {
		t.Error("Zustand existiert noch")
	}
	// Danach lässt sich das PRD wieder starten.
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Errorf("Neustart nach Teardown: %v", err)
	}
}

func TestTeardownWithoutState(t *testing.T) {
	_, target := setupProject(t)
	if err := Teardown(target, "new-token", io.Discard); err != nil {
		t.Errorf("Teardown ohne Lauf: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target.Dir(), "prd/auth/new-token.md")); err != nil {
		t.Error("Teardown ohne Lauf hat das PRD im Hauptordner angefasst")
	}
}

func TestRunRejectsUnknownAndImplemented(t *testing.T) {
	cfg, target := setupProject(t)
	if err := Run(cfg, target, "fehlt", io.Discard); err == nil {
		t.Error("unbekanntes PRD akzeptiert")
	}
	write(t, target.Dir(), "prd/auth/done.md", "# PRD: Fertig\n> Status: umgesetzt (2026-10-01)\n")
	if err := Run(cfg, target, "done", io.Discard); err == nil {
		t.Error("umgesetztes PRD akzeptiert")
	}
	if _, err := FindProject(cfg, "nope"); err == nil {
		t.Error("unbekanntes Projekt akzeptiert")
	}
}

func TestRunResumesFailedStep(t *testing.T) {
	cfg, target := setupProject(t)
	t.Setenv("FAKE_CLAUDE_MODE", "abort")
	err := Run(cfg, target, "new-token", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "Feld fehlt im PRD") {
		t.Fatalf("Abbruch der Umsetzung: %v", err)
	}
	r, _ := state.Load("app", "new-token")
	if r.Phase != project.PhaseError || NextStep(r) != StepImplement || r.Steps[1].OK {
		t.Errorf("Zustand nach Abbruch: %+v", r)
	}

	t.Setenv("FAKE_CLAUDE_MODE", "ok")
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	r, _ = state.Load("app", "new-token")
	names := []string{}
	for _, s := range r.Steps {
		names = append(names, s.Name)
	}
	if r.Phase != project.PhaseMergeQueue || r.Error != nil || strings.Join(names, ",") != "setup,implement,implement,review" {
		t.Errorf("Zustand nach Wiederaufnahme: %+v", r)
	}
}

func TestRunChecks(t *testing.T) {
	cases := []struct{ mode, want string }{
		{"exit", "Exit-Code 3"},
		{"noresult", "ohne result"},
		{"status", "Statuszeile"},
		{"dirty", "nicht sauber"},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			cfg, target := setupProject(t)
			t.Setenv("FAKE_CLAUDE_MODE", c.mode)
			err := Run(cfg, target, "new-token", io.Discard)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Fehler = %v, erwartet %q", err, c.want)
			}
		})
	}
}

func TestStartRespectsMaxParallel(t *testing.T) {
	cfg, target := setupProject(t)
	cfg.MaxParallel = 1
	var spawned []string
	pids := map[int]bool{}
	spawnRunner = func(id, prd string) (int, error) {
		spawned = append(spawned, prd)
		pid := 1000 + len(spawned)
		pids[pid] = true
		return pid, nil
	}
	isRunning = func(r *state.Run) bool { return pids[r.PID] }
	t.Cleanup(func() { isRunning = func(r *state.Run) bool { return r.Running() } })

	if started, err := Start(cfg, target, "new-token"); err != nil || !started {
		t.Fatalf("erster Start: %v, %v", started, err)
	}
	if started, err := Start(cfg, target, "tracked"); err != nil || started {
		t.Fatalf("zweiter Start: %v, %v", started, err)
	}
	if r, _ := state.Load("app", "tracked"); r.Phase != project.PhaseWaiting {
		t.Errorf("Phase des zweiten PRDs: %s", r.Phase)
	}
	if _, err := Start(cfg, target, "new-token"); err == nil {
		t.Error("doppelter Start akzeptiert")
	}

	// Der erste Runner endet in der Merge-Queue.
	first, _ := state.Load("app", "new-token")
	first.Phase, first.PID = project.PhaseMergeQueue, 0
	state.Save("app", "new-token", first)
	clear(pids)
	if err := StartWaiting(cfg); err != nil {
		t.Fatal(err)
	}
	if strings.Join(spawned, ",") != "new-token,tracked" {
		t.Errorf("gestartet: %v", spawned)
	}
	if r, _ := state.Load("app", "tracked"); !pids[r.PID] {
		t.Errorf("PID nicht gespeichert: %+v", r)
	}
}

// fakeClaude schreibt ein Skript, das claude -p nachahmt. Es erkennt den Schritt an der Kopfzeile
// "# lazydev <schritt>:" des Prompts. FAKE_CLAUDE_MODE steuert das Verhalten: ok, abort, exit,
// noresult, status (Statuszeile bleibt), dirty (Review committet nicht), lazydev (die Umsetzung
// legt eine Datei unter .lazydev/ an).
func fakeClaude(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	script := `#!/bin/sh
# printf statt echo, weil echo in dash "\n" als Zeilenumbruch ausgibt.
{ printf '%s\n' "$@"; echo "GIT_EDITOR=$GIT_EDITOR"; } >> .fake-args
echo '{"type":"system","subtype":"init","session_id":"sess-1","model":"claude-opus-5-5"}'
echo 'eine Warnung' >&2
mode="$FAKE_CLAUDE_MODE"
case "$2" in
"# lazydev implement: "*)
	prd=$(printf '%s\n' "$2" | sed -n '1s/^# lazydev implement: //p')
	case "$mode" in
	exit) echo '{"type":"result","is_error":true,"result":"kaputt"}'; exit 3 ;;
	noresult) exit 0 ;;
	abort) printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.2,"result":"## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- Feld fehlt im PRD\n"}'; exit 0 ;;
	status) ;;
	lazydev) mkdir -p .lazydev && echo x > .lazydev/neu.md
		sed -i 's/^> Status: geplant/> Status: umgesetzt (2026-10-20)/' "$prd" ;;
	*) sed -i 's/^> Status: geplant/> Status: umgesetzt (2026-10-20)/' "$prd" ;;
	esac
	echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"x.ts"}}]}}'
	printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.5,"result":"## Ergebnis\numgesetzt\n\n## Offene Fragen\n- keine\n"}'
	;;
"# lazydev merge: "*)
	if [ "$FAKE_MERGE_MODE" = abort ]; then
		printf '%s\n' '{"type":"result","is_error":false,"result":"## Ergebnis\nabgebrochen\n\n## Offene Punkte\n- review.txt: beide Seiten unklar\n"}'
		exit 0
	fi
	git rebase -q -X theirs main >/dev/null 2>&1 || true
	touch .merge-fixed
	printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.1,"result":"## Ergebnis\nbereit für Fast-Forward\n"}'
	;;
"# lazydev review: "*)
	echo change > review.txt
	if [ "$mode" != dirty ]; then
		git add -A && git commit -q -m "feat: review"
	fi
	echo '{"type":"result","is_error":false,"total_cost_usd":0.3,"result":"fertig"}'
	;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitRepo legt Dateien unter .lazydev/ und weitere im Hauptordner an und committet nur sie. Das
// ungetrackte PRD bleibt ungetrackt.
func commitRepo(t *testing.T, target Target, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		write(t, target.Dir(), rel, content)
		runGit(t, target.Dir(), "add", "--", rel)
	}
	runGit(t, target.Dir(), "commit", "-q", "-m", "lazydev")
}

func TestRunUsesProjectPrompt(t *testing.T) {
	cfg, target := setupProject(t)
	commitRepo(t, target, map[string]string{
		".lazydev/config.yaml":  "knowledge:\n  all: [CLAUDE.md]\n  review: [docs/review.md]\n",
		".lazydev/implement.md": "---\ndescription: Team\n---\n# Ablauf des Teams\nSetze $ARGUMENTS auf $BRANCH um.\n",
		"CLAUDE.md":             "# Regeln\n",
		"docs/review.md":        "# Review\n",
	})
	// Hooks aus der zentralen config.yaml brauchen keine Bestätigung, sie wirken hier aber nicht.
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	wt := worktree.Path("app", "new-token")
	args, _ := os.ReadFile(filepath.Join(wt, ".fake-args"))
	want := "# lazydev implement: prd/auth/new-token.md\n\n## Pflichtlektüre\n\nLies diese Dateien vollständig, bevor du etwas anderes tust:\n\n- `CLAUDE.md`\n\n" +
		"# Ablauf des Teams\nSetze prd/auth/new-token.md auf feat/new-token um.\n\n## Regeln von lazydev"
	if !strings.Contains(string(args), want) {
		t.Errorf("Aufruf von claude ohne Projekt-Prompt:\n%s", args)
	}
	implLog, _ := os.ReadFile(state.LogPath("app", "new-token", StepImplement))
	if !strings.Contains(string(implLog), "Prompt: .lazydev/implement.md") || !strings.Contains(string(implLog), "Knowledge: CLAUDE.md") {
		t.Errorf("Log von implement:\n%s", implLog)
	}
	reviewLog, _ := os.ReadFile(state.LogPath("app", "new-token", StepReview))
	if !strings.Contains(string(reviewLog), "Prompt: Standard") || !strings.Contains(string(reviewLog), "Knowledge: CLAUDE.md, docs/review.md") {
		t.Errorf("Log von review:\n%s", reviewLog)
	}
	// Mit .lazydev/config.yaml wirken die Hooks der zentralen config.yaml nicht.
	if _, err := os.Stat(filepath.Join(wt, "port.txt")); err == nil {
		t.Error("Setup-Skript der zentralen config.yaml lief")
	}
}

func TestRunMissingKnowledgeSkipsClaude(t *testing.T) {
	cfg, target := setupProject(t)
	commitRepo(t, target, map[string]string{".lazydev/config.yaml": "knowledge:\n  implement: [LOKAL.md]\n"})
	// Nur im Hauptordner, nicht im Worktree.
	write(t, target.Dir(), "LOKAL.md", "x")
	err := Run(cfg, target, "new-token", io.Discard)
	if err == nil || err.Error() != "Knowledge-Datei fehlt: LOKAL.md" {
		t.Fatalf("Fehler: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree.Path("app", "new-token"), ".fake-args")); err == nil {
		t.Error("claude lief trotz fehlender Knowledge-Datei")
	}
	if r, _ := state.Load("app", "new-token"); r.Phase != project.PhaseError || NextStep(r) != StepImplement {
		t.Errorf("Zustand: %+v", r)
	}
}

func TestRunFailsWhenClaudeChangesLazydev(t *testing.T) {
	cfg, target := setupProject(t)
	t.Setenv("FAKE_CLAUDE_MODE", "lazydev")
	err := Run(cfg, target, "new-token", io.Discard)
	if err == nil || err.Error() != ".lazydev/ im Worktree geändert: .lazydev/neu.md" {
		t.Fatalf("Fehler: %v", err)
	}
	if r, _ := state.Load("app", "new-token"); NextStep(r) != StepImplement {
		t.Errorf("Zustand: %+v", r)
	}
}

func TestRunNeedsTrust(t *testing.T) {
	cfg, target := setupProject(t)
	commitRepo(t, target, map[string]string{".lazydev/config.yaml": "hooks:\n  setup: ./setup.sh\n"})
	const hint = "Hooks oder Freigaben in .lazydev/config.yaml nicht bestätigt. Prüfen und bestätigen: lazydev trust app"
	if err := Run(cfg, target, "new-token", io.Discard); err == nil || err.Error() != hint {
		t.Fatalf("Run ohne Vertrauen: %v", err)
	}
	if _, err := Start(cfg, target, "new-token"); err == nil || err.Error() != hint {
		t.Errorf("Start ohne Vertrauen: %v", err)
	}
	if _, err := state.Load("app", "new-token"); err == nil {
		t.Error("Lauf ohne Vertrauen angelegt")
	}

	repo, _, _ := config.LoadRepo(target.Dir())
	if err := state.Trust("app", repo.TrustHash()); err != nil {
		t.Fatal(err)
	}
	if err := Run(cfg, target, "new-token", io.Discard); err != nil {
		t.Fatal(err)
	}
	if port, _ := os.ReadFile(filepath.Join(worktree.Path("app", "new-token"), "port.txt")); len(port) == 0 {
		t.Error("Setup-Hook aus .lazydev/config.yaml lief nicht")
	}

	// Eine Änderung an den Freigaben verlangt erneut eine Bestätigung, auch uncommittet.
	write(t, target.Dir(), ".lazydev/config.yaml", "hooks:\n  setup: ./setup.sh\nallowedTools: [\"Bash(make:*)\"]\n")
	if err := Run(cfg, target, "tracked", io.Discard); err == nil || err.Error() != hint {
		t.Errorf("Run nach Änderung: %v", err)
	}
}
