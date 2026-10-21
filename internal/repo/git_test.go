package repo

import (
	"slices"
	"testing"
)

func TestLocalBranches(t *testing.T) {
	dir := initRepo(t, "main")
	commitAll(t, dir, "init")
	for _, b := range []string{"develop", "feat/x", "feature/y", "fix/feat/z"} {
		gitRun(t, dir, "branch", b)
	}
	got, err := LocalBranches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"develop", "feature/y", "fix/feat/z", "main"}; !slices.Equal(got, want) {
		t.Errorf("LocalBranches = %v, want %v", got, want)
	}
	if _, err := LocalBranches(t.TempDir()); err == nil {
		t.Error("no error outside git")
	}
}

func TestKnowledgeCandidates(t *testing.T) {
	dir := initRepo(t, "main")
	for _, f := range []string{"README.md", "docs/z.md", "docs/a.md", "prd/a/x.md", ".lazydev/implement.md", "prdx/b.md", "notes.txt"} {
		writeFile(t, dir, f, "x\n")
	}
	commitAll(t, dir, "init")
	writeFile(t, dir, "untracked.md", "x\n")
	got, err := KnowledgeCandidates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md", "docs/a.md", "docs/z.md", "prdx/b.md"}; !slices.Equal(got, want) {
		t.Errorf("KnowledgeCandidates = %v, want %v", got, want)
	}
	if _, err := KnowledgeCandidates(t.TempDir()); err == nil {
		t.Error("no error outside git")
	}
}

func TestDetectBaseBranch(t *testing.T) {
	t.Run("main", func(t *testing.T) {
		dir := initRepo(t, "main")
		commitAll(t, dir, "init")
		gitRun(t, dir, "branch", "master")
		gitRun(t, dir, "checkout", "-q", "-b", "topic")
		if got := DetectBaseBranch(dir); got != "main" {
			t.Errorf("DetectBaseBranch = %q", got)
		}
	})
	t.Run("master", func(t *testing.T) {
		dir := initRepo(t, "master")
		commitAll(t, dir, "init")
		gitRun(t, dir, "checkout", "-q", "-b", "topic")
		if got := DetectBaseBranch(dir); got != "master" {
			t.Errorf("DetectBaseBranch = %q", got)
		}
	})
	t.Run("current", func(t *testing.T) {
		dir := initRepo(t, "develop")
		commitAll(t, dir, "init")
		gitRun(t, dir, "checkout", "-q", "-b", "topic")
		if got := DetectBaseBranch(dir); got != "topic" {
			t.Errorf("DetectBaseBranch = %q", got)
		}
	})
	t.Run("without commits", func(t *testing.T) {
		dir := initRepo(t, "trunk")
		if got := DetectBaseBranch(dir); got != "trunk" {
			t.Errorf("DetectBaseBranch = %q", got)
		}
	})
	t.Run("outside git", func(t *testing.T) {
		setupEnv(t)
		if got := DetectBaseBranch(t.TempDir()); got != "" {
			t.Errorf("DetectBaseBranch = %q", got)
		}
	})
}
