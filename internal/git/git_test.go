package git

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func newRepo(t *testing.T) string {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
	} {
		t.Setenv(k, v)
	}
	dir := t.TempDir()
	if _, err := Run(dir, "init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	if _, err := Run(dir, "add", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, "commit", "-q", "-m", "init"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunError(t *testing.T) {
	dir := newRepo(t)
	_, err := Run(dir, "rev-parse", "--verify", "nope")
	if err == nil {
		t.Fatal("no error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "git rev-parse --verify nope: ") || !strings.Contains(msg, "fatal") {
		t.Errorf("error: %q", msg)
	}
}

func TestRunOutput(t *testing.T) {
	dir := newRepo(t)
	out, err := Run(dir, "log", "--format=%s")
	if err != nil || out != "init" {
		t.Errorf("Run: %q, %v", out, err)
	}
	// Status porcelain keeps its leading space.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "ü.txt"), nil, 0o644)
	lines := Output(dir, "status", "--porcelain")
	if !slices.Equal(lines, []string{" M a.txt", "?? ü.txt"}) {
		t.Errorf("Output: %q", lines)
	}
	if Output(dir, "rev-parse", "--verify", "nope") != nil {
		t.Error("Output on error is not nil")
	}
}

func TestLines(t *testing.T) {
	if got := Lines(" M a\n\n?? b\n"); !slices.Equal(got, []string{" M a", "?? b"}) {
		t.Errorf("Lines: %q", got)
	}
	if got := Lines(""); got != nil {
		t.Errorf("empty: %q", got)
	}
}

func TestIsTrackedBranchCurrent(t *testing.T) {
	dir := newRepo(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), nil, 0o644)
	if !IsTracked(dir, "a.txt") || IsTracked(dir, "new.txt") || IsTracked(dir, "missing") {
		t.Error("IsTracked")
	}
	if !BranchExists(dir, "main") || BranchExists(dir, "feat/x") {
		t.Error("BranchExists before branch")
	}
	if _, err := Run(dir, "switch", "-q", "-c", "feat/x"); err != nil {
		t.Fatal(err)
	}
	if !BranchExists(dir, "feat/x") || CurrentBranch(dir) != "feat/x" {
		t.Errorf("after switch: %q", CurrentBranch(dir))
	}
	Run(dir, "switch", "-q", "--detach")
	if got := CurrentBranch(dir); got != "" {
		t.Errorf("detached: %q", got)
	}
	if got := CurrentBranch(t.TempDir()); got != "" {
		t.Errorf("outside a repository: %q", got)
	}
}
