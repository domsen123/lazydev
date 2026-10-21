package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"lazydev/internal/claude"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

const prdName = "new-token"

const prdPath = "prd/auth/new-token.md"

const prdContent = "# PRD: Token\n\n> Status: geplant · Scope: tokens\n\nbody\n"

// hooksConfig is a config.yaml with all hooks, an allowed tool and knowledge. It needs trust.
const hooksConfig = `knowledge:
  all:
    - CLAUDE.md
allowedTools:
  - Bash(make test:*)
hooks:
  setup: setup.sh
  check: check.sh
  teardown: teardown.sh
`

// Hooks append a line to hooks.log in the main folder, so the test sees them after the worktree
// is gone. FAKE_CHECK=fail makes the check red, FAKE_CHECK=once only the first time.
const (
	setupScript = `#!/bin/sh
echo "setup $(pwd -P) $LAZYDEV_PORT $LAZYDEV_SLUG $LAZYDEV_BRANCH" >> "$LAZYDEV_PROJECT/hooks.log"
echo "setup output"
`
	checkScript = `#!/bin/sh
echo "check $(pwd -P)" >> "$LAZYDEV_PROJECT/hooks.log"
echo "check output"
case "$FAKE_CHECK" in
fail) exit 1 ;;
once) if [ ! -f "$LAZYDEV_PROJECT/.checked" ]; then touch "$LAZYDEV_PROJECT/.checked"; exit 1; fi ;;
esac
`
	teardownScript = `#!/bin/sh
echo "teardown $(pwd -P)" >> "$LAZYDEV_PROJECT/hooks.log"
`
)

// fakeScript imitates claude -p. It recognizes the step by the header "# lazydev <step>:" in the
// first line of the prompt in $2 and appends its arguments to $FAKE_ARGS. FAKE_CLAUDE_MODE: ok,
// exit, noresult, error, abort, status (status line stays), dirty (review does not commit),
// lazydev (implement creates .lazydev/neu.md), sleep (implement sleeps until a signal ends it).
// FAKE_MERGE_MODE: ok (rebase -X theirs), noop (no rebase), abort.
const fakeScript = `#!/bin/sh
# printf instead of echo, dash's echo turns "\n" into a newline.
{ printf '%s\n' "$@"; echo "GIT_EDITOR=$GIT_EDITOR"; echo "PWD=$(pwd -P)"; } >> "$FAKE_ARGS"
echo '{"type":"system","subtype":"init","session_id":"sess-1","model":"claude-opus-5-5"}'
echo 'eine Warnung' >&2
mode="$FAKE_CLAUDE_MODE"
case "$2" in
"# lazydev implement: "*)
	prd=$(printf '%s\n' "$2" | sed -n '1s/^# lazydev implement: //p')
	case "$mode" in
	exit) echo '{"type":"result","is_error":true,"result":"kaputt"}'; exit 3 ;;
	noresult) exit 0 ;;
	error) echo '{"type":"result","is_error":true,"result":"Rate limit"}'; exit 0 ;;
	abort) printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.2,"result":"## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- Feld fehlt im PRD\n"}'; exit 0 ;;
	status) ;;
	sleep) exec sleep 30 ;;
	lazydev) mkdir -p .lazydev && echo x > .lazydev/neu.md
		sed -i 's/^> Status: geplant/> Status: umgesetzt (2026-10-20)/' "$prd" ;;
	*) sed -i 's/^> Status: geplant/> Status: umgesetzt (2026-10-20)/' "$prd" ;;
	esac
	echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"x.ts"}}]}}'
	printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.5,"result":"## Ergebnis\numgesetzt\n\n## Offene Fragen\n- keine\n"}'
	;;
"# lazydev review: "*)
	echo change > review.txt
	if [ "$mode" != dirty ]; then git add -A && git commit -q -m "feat: review"; fi
	echo '{"type":"result","is_error":false,"total_cost_usd":0.3,"result":"fertig"}'
	;;
"# lazydev merge: "*)
	case "$FAKE_MERGE_MODE" in
	noop) ;;
	abort)
		printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.1,"result":"## Ergebnis\nabgebrochen\n\n## Offene Punkte\n- Konflikt in review.txt\n"}'
		exit 0 ;;
	*) git rebase -q -X theirs main >/dev/null 2>&1 || true ;;
	esac
	printf '%s\n' '{"type":"result","is_error":false,"total_cost_usd":0.1,"result":"## Ergebnis\nbereit für Fast-Forward\n"}'
	;;
esac
`

// isolate points state, data and config at temporary folders, sets the git identity and replaces
// claude with the fake. Runners never spawn: Executable fails.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	t.Setenv("FAKE_CLAUDE_MODE", "ok")
	t.Setenv("FAKE_MERGE_MODE", "ok")
	t.Setenv("FAKE_CHECK", "")

	bin := t.TempDir()
	fake := filepath.Join(bin, "claude")
	if err := os.WriteFile(fake, []byte(fakeScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_ARGS", filepath.Join(bin, "args"))

	oldCommand, oldPoll, oldExe := claude.Command, MergePollInterval, Executable
	claude.Command = []string{fake}
	MergePollInterval = 10 * time.Millisecond
	Executable = func() (string, error) { return "", errors.New("no runner spawns in tests") }
	t.Cleanup(func() { claude.Command, MergePollInterval, Executable = oldCommand, oldPoll, oldExe })
}

// newProject creates a committed repo with .lazydev/ (default prompts, config.yaml as given) and
// the hook scripts, plus the untracked PRD prd/auth/new-token.md. With trust, the hash of
// config.yaml is confirmed.
func newProject(t *testing.T, configYAML string, trust bool) Target {
	t.Helper()
	isolate(t)
	dir := filepath.Join(t.TempDir(), "app")
	write(t, dir, "CLAUDE.md", "# app\n")
	write(t, dir, ".gitignore", "hooks.log\n.checked\n")
	for name, script := range map[string]string{"setup.sh": setupScript, "check.sh": checkScript, "teardown.sh": teardownScript} {
		write(t, dir, name, script)
		if err := os.Chmod(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, dir, "init", "-q", "-b", "main")
	if err := repo.Init(dir); err != nil {
		t.Fatal(err)
	}
	if configYAML != "" {
		write(t, dir, ".lazydev/config.yaml", configYAML)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	write(t, dir, prdPath, prdContent)
	target := Target{ID: "app", Dir: dir}
	if trust {
		trustProject(t, target)
	}
	return target
}

func trustProject(t *testing.T, target Target) {
	t.Helper()
	if err := state.Trust(target.ID, repo.Read(target.Dir, target.ID).Config.TrustHash()); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func loadRun(t *testing.T) *state.Run {
	t.Helper()
	r, err := state.Load("app", prdName)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func fakeArgs(t *testing.T) string {
	t.Helper()
	data, _ := os.ReadFile(os.Getenv("FAKE_ARGS"))
	return string(data)
}

// startFakeRunner starts a process whose command line is "lazydev run", so state.IsRunner counts
// it as a living runner: a symlink named lazydev to sh runs the script "run" in its folder.
func startFakeRunner(t *testing.T) int {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	if err := os.Symlink(sh, filepath.Join(dir, state.BinaryName)); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "run", "sleep 30\n")
	cmd := exec.Command(filepath.Join(dir, state.BinaryName), "run")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Wait()
	})
	waitFor(t, "the fake runner", func() bool { return state.IsRunner(cmd.Process.Pid) })
	return cmd.Process.Pid
}

// waitFor polls cond until it holds or the timeout ends.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
