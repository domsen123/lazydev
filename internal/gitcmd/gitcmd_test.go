package gitcmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/state"
)

const id = "shop"

// setup creates a bare repository as origin and a repository on main with one commit and origin as
// remote, but without upstream.
func setup(t *testing.T) (bare, work string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
	} {
		t.Setenv(k, v)
	}
	root := t.TempDir()
	bare, work = filepath.Join(root, "origin.git"), filepath.Join(root, "shop")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", bare)
	gitIn(t, root, "init", "-q", "-b", "main", work)
	gitIn(t, work, "remote", "add", "origin", bare)
	write(t, work, "README.md", "# shop\n")
	gitIn(t, work, "add", ".")
	gitIn(t, work, "commit", "-q", "-m", "init")
	return bare, work
}

// setupPushed is setup with main pushed to origin and tracked as upstream.
func setupPushed(t *testing.T) (bare, work string) {
	t.Helper()
	bare, work = setup(t)
	gitIn(t, work, "push", "-q", "-u", "origin", "main")
	return bare, work
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitIn commits a new file name in dir.
func commitIn(t *testing.T, dir, name string) {
	t.Helper()
	write(t, dir, name, name+"\n")
	gitIn(t, dir, "add", name)
	gitIn(t, dir, "commit", "-q", "-m", "add "+name)
}

func runAction(t *testing.T, dir, action, message string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := Run(id, dir, action, message, nil, &out)
	return code, out.String()
}

func TestCommit(t *testing.T) {
	_, work := setup(t)
	write(t, work, "README.md", "# shop\n\nchanged\n")
	write(t, work, "new.txt", "new\n")
	message := "feat: add new file\n\nWith a body."
	code, out := runAction(t, work, Commit, message)
	if code != 0 {
		t.Fatalf("code %d: %s", code, out)
	}
	if !strings.Contains(out, "$ git add -A") {
		t.Errorf("output: %s", out)
	}
	if status := gitIn(t, work, "status", "--porcelain"); status != "" {
		t.Errorf("status after commit: %q", status)
	}
	if got := gitIn(t, work, "log", "-1", "--format=%B"); got != message {
		t.Errorf("message: %q", got)
	}
}

func TestCommitEmptyMessage(t *testing.T) {
	_, work := setup(t)
	write(t, work, "new.txt", "new\n")
	code, out := runAction(t, work, Commit, "  ")
	if code != 1 || !strings.Contains(out, "Enter a commit message.") {
		t.Errorf("code %d: %s", code, out)
	}
	if n := gitIn(t, work, "rev-list", "--count", "HEAD"); n != "1" {
		t.Errorf("commits: %s", n)
	}
}

func TestCommitNothing(t *testing.T) {
	_, work := setup(t)
	if code, out := runAction(t, work, Commit, "chore: nothing"); code == 0 {
		t.Errorf("code 0: %s", out)
	}
}

func TestPushWithoutUpstream(t *testing.T) {
	_, work := setup(t)
	code, out := runAction(t, work, Push, "")
	if code != 0 || !strings.Contains(out, "$ git push -u origin main") {
		t.Fatalf("code %d: %s", code, out)
	}
	if up := gitIn(t, work, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); up != "origin/main" {
		t.Errorf("upstream: %q", up)
	}
}

func TestPushWithUpstream(t *testing.T) {
	_, work := setupPushed(t)
	commitIn(t, work, "a.txt")
	code, out := runAction(t, work, Push, "")
	if code != 0 || !strings.Contains(out, "$ git push\n") || strings.Contains(out, "-u") {
		t.Errorf("code %d: %s", code, out)
	}
}

func TestPull(t *testing.T) {
	bare, work := setupPushed(t)
	other := filepath.Join(t.TempDir(), "other")
	gitIn(t, work, "clone", "-q", bare, other)
	commitIn(t, other, "b.txt")
	gitIn(t, other, "push", "-q")
	code, out := runAction(t, work, Pull, "")
	if code != 0 || !strings.Contains(out, "$ git pull --ff-only") {
		t.Fatalf("code %d: %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(work, "b.txt")); err != nil {
		t.Errorf("pulled file: %v", err)
	}
}

func TestPullDiverged(t *testing.T) {
	bare, work := setupPushed(t)
	other := filepath.Join(t.TempDir(), "other")
	gitIn(t, work, "clone", "-q", bare, other)
	commitIn(t, other, "b.txt")
	gitIn(t, other, "push", "-q")
	commitIn(t, work, "c.txt")
	if code, out := runAction(t, work, Pull, ""); code == 0 {
		t.Errorf("code 0: %s", out)
	}
	if merges := gitIn(t, work, "log", "--merges", "--oneline"); merges != "" {
		t.Errorf("merge commits: %s", merges)
	}
}

func TestDetachedHead(t *testing.T) {
	_, work := setupPushed(t)
	gitIn(t, work, "checkout", "-q", "--detach")
	for _, action := range []string{Pull, Push} {
		code, out := runAction(t, work, action, "")
		if code != 1 || !strings.Contains(out, "HEAD is detached, check out a branch first.") {
			t.Errorf("%s: code %d: %s", action, code, out)
		}
	}
}

func TestMergeLockHeld(t *testing.T) {
	_, work := setupPushed(t)
	unlock, err := state.Lock(state.MergeLock(id))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	for _, action := range []string{Pull, Push} {
		code, out := runAction(t, work, action, "")
		if code != 1 || !strings.Contains(out, "merges right now") || strings.Contains(out, "$ git") {
			t.Errorf("%s: code %d: %s", action, code, out)
		}
	}
	if code, out := runAction(t, work, Fetch, ""); code != 0 {
		t.Errorf("fetch: code %d: %s", code, out)
	}
}

func TestUnknownAction(t *testing.T) {
	_, work := setup(t)
	if code, out := runAction(t, work, "rebase", ""); code != 1 || !strings.Contains(out, "unknown git action: rebase") {
		t.Errorf("code %d: %s", code, out)
	}
}
