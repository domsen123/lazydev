package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// initRepo creates a git repository on main without commits.
func initRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	return dir
}

// gitRun runs git in dir and fails the test on errors.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeFile writes content to rel below dir and creates missing folders.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitAll commits everything in dir.
func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", msg)
}

func TestRead(t *testing.T) {
	tests := []struct {
		name, rel, content string
		want               PRD
	}{
		{
			name:    "status after blank line",
			rel:     "prd/auth/token.md",
			content: "# PRD: Token refresh\n\n> Status: geplant · Scope: backend\n\n## Goal\n",
			want: PRD{Path: "prd/auth/token.md", Name: "token", Area: "auth", Title: "Token refresh",
				Status: Status{Kind: StatusPlanned, Value: "geplant", Detail: "Scope: backend"}},
		},
		{
			name:    "multi-line detail",
			rel:     "prd/ui/theme.md",
			content: "# PRD: Theme\n> Status: umgesetzt (2026-10-14) · Kosten: 1.50 USD · Modell: opus\n> second line\n>  third\n\n> other block\n",
			want: PRD{Path: "prd/ui/theme.md", Name: "theme", Area: "ui", Title: "Theme",
				Status: Status{Kind: StatusImplemented, Value: "umgesetzt (2026-10-14)", Date: "2026-10-14",
					Detail: "Kosten: 1.50 USD · Modell: opus\nsecond line\n third"},
				Cost: 1.5, HasCost: true, Model: "opus"},
		},
		{
			name:    "no area, title without prefix",
			rel:     "prd/top.md",
			content: "# Notes\n\nText\n",
			want:    PRD{Path: "prd/top.md", Name: "top", Title: "Notes"},
		},
		{
			name:    "nested area",
			rel:     "prd/a/b/deep.md",
			content: "# PRD: Deep\n> Status: verworfen (2026-10-21)\n",
			want: PRD{Path: "prd/a/b/deep.md", Name: "deep", Area: "a/b", Title: "Deep",
				Status: Status{Kind: StatusDiscarded, Value: "verworfen (2026-10-21)", Date: "2026-10-21"}},
		},
		{
			name:    "status after the head",
			rel:     "prd/x/late.md",
			content: "# PRD: Late\n" + strings.Repeat("text\n", HeaderScanLines) + "> Status: geplant\n",
			want:    PRD{Path: "prd/x/late.md", Name: "late", Area: "x", Title: "Late"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, tt.rel, tt.content)
			d, err := Read(dir, filepath.Join(dir, tt.rel))
			if err != nil {
				t.Fatal(err)
			}
			if d.Created.IsZero() {
				t.Error("Created is zero")
			}
			tt.want.Root = dir
			d.Created = time.Time{}
			if d != tt.want {
				t.Errorf("Read =\n%+v\nwant\n%+v", d, tt.want)
			}
		})
	}
}

func TestReadMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := Read(dir, filepath.Join(dir, "prd/x.md")); err == nil {
		t.Error("no error for a missing file")
	}
}

func TestLabel(t *testing.T) {
	if l := (PRD{Name: "token", Area: "auth"}).Label(); l != "auth/token" {
		t.Errorf("Label = %q", l)
	}
	if l := (PRD{Name: "token"}).Label(); l != "token" {
		t.Errorf("Label without area = %q", l)
	}
}

func TestPRDPhase(t *testing.T) {
	for kind, want := range map[StatusKind]Phase{
		StatusPlanned:     PhasePlanned,
		StatusImplemented: PhaseMerged,
		StatusDiscarded:   PhaseDiscarded,
		StatusUnknown:     PhaseUnknown,
	} {
		if got := (PRD{Status: Status{Kind: kind}}).Phase(); got != want {
			t.Errorf("Phase(%v) = %s, want %s", kind, got, want)
		}
	}
}

func TestPhaseMethods(t *testing.T) {
	tests := []struct {
		phase        Phase
		done, active bool
		symbol       string
	}{
		{PhasePlanned, false, false, "○"},
		{PhaseWaiting, false, false, "◌"},
		{PhaseSetup, false, true, "●"},
		{PhaseImplement, false, true, "●"},
		{PhaseReview, false, true, "●"},
		{PhaseMergeQueue, false, false, "◐"},
		{PhaseMerge, false, true, "●"},
		{PhaseMerged, true, false, "✓"},
		{PhaseAborted, false, false, "■"},
		{PhaseError, false, false, "✗"},
		{PhaseDiscarded, true, false, "✗"},
		{PhaseUnknown, false, false, "?"},
		{Phase("other"), false, false, "?"},
	}
	for _, tt := range tests {
		if tt.phase.Done() != tt.done || tt.phase.Active() != tt.active || tt.phase.Symbol() != tt.symbol {
			t.Errorf("%s: Done=%v Active=%v Symbol=%q", tt.phase, tt.phase.Done(), tt.phase.Active(), tt.phase.Symbol())
		}
	}
}

func TestReadAllWithoutPRDFolder(t *testing.T) {
	dir := initRepo(t)
	prds, err := ReadAll(dir, "main", nil)
	if err != nil || len(prds) != 0 {
		t.Errorf("ReadAll = %v, %v", prds, err)
	}
	// Outside git too.
	prds, err = ReadAll(t.TempDir(), "main", nil)
	if err != nil || len(prds) != 0 {
		t.Errorf("ReadAll outside git = %v, %v", prds, err)
	}
}

func TestReadAll(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# Readme\n")
	writeFile(t, dir, "prd/b/done.md", "# PRD: Done\n\n> Status: umgesetzt (2026-10-14)\n")
	writeFile(t, dir, "prd/a/feature.md", "# PRD: Feature\n\n> Status: geplant\n")
	writeFile(t, dir, "prd/a/notes.txt", "not a PRD\n")
	gitRun(t, dir, "add", "-A")
	old := "2020-01-02T03:04:05Z"
	cmd := exec.Command("git", "-C", dir, "commit", "-q", "-m", "init")
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+old, "GIT_AUTHOR_DATE="+old)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
	gitRun(t, dir, "branch", "feat/feature")
	writeFile(t, dir, "prd/c/new.md", "# PRD: New\n\n> Status: geplant\n")

	prds, err := ReadAll(dir, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, d := range prds {
		paths = append(paths, d.Path)
	}
	if strings.Join(paths, ",") != "prd/a/feature.md,prd/b/done.md,prd/c/new.md" {
		t.Fatalf("paths = %v", paths)
	}
	feature, done, untracked := prds[0], prds[1], prds[2]
	if !feature.Tracked || feature.Branch != "feat/feature" || feature.Root != dir {
		t.Errorf("feature: %+v", feature)
	}
	if !done.Tracked || done.Branch != "main" {
		t.Errorf("done: %+v", done)
	}
	if untracked.Tracked || untracked.Branch != "" {
		t.Errorf("untracked: %+v", untracked)
	}
	want, _ := time.Parse(time.RFC3339, old)
	if !feature.Created.Equal(want) || !done.Created.Equal(want) {
		t.Errorf("Created = %v, %v; want %v", feature.Created, done.Created, want)
	}
	if untracked.Created.IsZero() || untracked.Created.Before(want.Add(time.Hour)) {
		t.Errorf("untracked Created = %v", untracked.Created)
	}

	// The base branch name passes through for merged PRDs.
	prds, _ = ReadAll(dir, "develop", nil)
	if prds[1].Branch != "develop" {
		t.Errorf("Branch with base develop = %q", prds[1].Branch)
	}
}

func TestReadAllFeatureBranchWinsForMerged(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "prd/a/x.md", "# PRD: X\n> Status: umgesetzt (2026-10-14)\n")
	commitAll(t, dir, "init")
	gitRun(t, dir, "branch", "feat/x")
	prds, err := ReadAll(dir, "main", nil)
	if err != nil || len(prds) != 1 || prds[0].Branch != "feat/x" {
		t.Errorf("ReadAll = %+v, %v", prds, err)
	}
}

func TestReadAllMoved(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "prd/a/present.md", "# PRD: Present main\n> Status: geplant\n")
	commitAll(t, dir, "init")

	wt := t.TempDir()
	writeFile(t, wt, "prd/b/gone.md", "# PRD: Gone\n> Status: geplant · Scope: wt\n")
	writeFile(t, wt, "prd/a/present.md", "# PRD: Present worktree\n> Status: geplant\n")
	moved := []Moved{
		{Name: "gone", Worktree: wt, Path: "prd/b/gone.md", Branch: "feat/gone"},
		{Name: "present", Worktree: wt, Path: "prd/a/present.md", Branch: "feat/present"},
		{Name: "missing", Worktree: wt, Path: "prd/c/missing.md", Branch: "feat/missing"},
	}
	prds, err := ReadAll(dir, "main", moved)
	if err != nil {
		t.Fatal(err)
	}
	if len(prds) != 2 {
		t.Fatalf("ReadAll = %+v", prds)
	}
	present, gone := prds[0], prds[1]
	if present.Title != "Present main" || present.Root != dir || present.Branch != "" {
		t.Errorf("present: %+v", present)
	}
	if gone.Path != "prd/b/gone.md" || gone.Root != wt || !gone.Tracked || gone.Branch != "feat/gone" ||
		gone.Area != "b" || gone.Status.Detail != "Scope: wt" {
		t.Errorf("gone: %+v", gone)
	}
}
