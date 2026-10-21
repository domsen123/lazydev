package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var changeDay = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

const changePRD = "# PRD: X\n\n> Status: geplant · Scope: a\n\nBody\n"

func choice(t *testing.T, kind StatusKind) Choice {
	t.Helper()
	for _, c := range Choices {
		if c.Kind == kind {
			return c
		}
	}
	t.Fatalf("no choice for %v", kind)
	return Choice{}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestChoiceValueAt(t *testing.T) {
	want := map[StatusKind]string{
		StatusPlanned:     "geplant",
		StatusImplemented: "umgesetzt (2026-10-21)",
		StatusDiscarded:   "verworfen (2026-10-21)",
	}
	if len(Choices) != 3 {
		t.Fatalf("Choices = %+v", Choices)
	}
	for _, c := range Choices {
		if got := c.ValueAt(changeDay); got != want[c.Kind] {
			t.Errorf("%s: ValueAt = %q", c.Label, got)
		}
		// The value parses back to the same kind.
		if s, _ := ParseStatusLine("> Status: " + c.ValueAt(changeDay)); s.Kind != c.Kind {
			t.Errorf("%s: parses as %v", c.Label, s.Kind)
		}
	}
}

func TestChangeStatusUntracked(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "x\n")
	commitAll(t, dir, "init")
	writeFile(t, dir, "prd/a/x.md", changePRD)
	head := gitRun(t, dir, "rev-parse", "HEAD")

	committed, err := ChangeStatus(dir, "main", "prd/a/x.md", choice(t, StatusImplemented), changeDay)
	if err != nil || committed {
		t.Fatalf("ChangeStatus = %v, %v", committed, err)
	}
	want := "# PRD: X\n\n> Status: umgesetzt (2026-10-21) · Scope: a\n\nBody\n"
	if got := readFile(t, filepath.Join(dir, "prd/a/x.md")); got != want {
		t.Errorf("file = %q", got)
	}
	if gitRun(t, dir, "rev-parse", "HEAD") != head {
		t.Error("untracked PRD got committed")
	}
}

func TestChangeStatusUntrackedOutsideGit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "prd/a/x.md", changePRD)
	committed, err := ChangeStatus(dir, "main", "prd/a/x.md", choice(t, StatusDiscarded), changeDay)
	if err != nil || committed {
		t.Fatalf("ChangeStatus = %v, %v", committed, err)
	}
	if got := readFile(t, filepath.Join(dir, "prd/a/x.md")); !strings.Contains(got, "> Status: verworfen (2026-10-21) · Scope: a\n") {
		t.Errorf("file = %q", got)
	}
}

func TestChangeStatusTracked(t *testing.T) {
	for _, tt := range []struct {
		kind   StatusKind
		start  string
		word   string
		status string
	}{
		{StatusImplemented, "geplant", "done", "umgesetzt (2026-10-21)"},
		{StatusPlanned, "umgesetzt (2026-10-01)", "planned", "geplant"},
		{StatusDiscarded, "geplant", "discarded", "verworfen (2026-10-21)"},
	} {
		t.Run(tt.word, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "prd/a/x.md", "# PRD: X\n\n> Status: "+tt.start+" · Scope: a\n")
			writeFile(t, dir, "other.md", "old\n")
			commitAll(t, dir, "init")
			// Other changes, staged or not, stay out of the commit.
			writeFile(t, dir, "other.md", "new\n")
			writeFile(t, dir, "staged.md", "staged\n")
			gitRun(t, dir, "add", "staged.md")

			committed, err := ChangeStatus(dir, "main", "prd/a/x.md", choice(t, tt.kind), changeDay)
			if err != nil || !committed {
				t.Fatalf("ChangeStatus = %v, %v", committed, err)
			}
			if msg := gitRun(t, dir, "log", "-1", "--format=%B"); msg != "docs(prd): mark x as "+tt.word {
				t.Errorf("commit message = %q", msg)
			}
			if files := gitRun(t, dir, "show", "--name-only", "--format=", "HEAD"); files != "prd/a/x.md" {
				t.Errorf("committed files = %q", files)
			}
			if got := gitRun(t, dir, "show", "HEAD:prd/a/x.md"); got != "# PRD: X\n\n> Status: "+tt.status+" · Scope: a" {
				t.Errorf("committed content = %q", got)
			}
			if st := gitRun(t, dir, "status", "--porcelain"); st != "M other.md\nA  staged.md" {
				t.Errorf("status = %q", st)
			}
		})
	}
}

func TestChangeStatusWrongBranch(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "prd/a/x.md", changePRD)
	commitAll(t, dir, "init")
	gitRun(t, dir, "checkout", "-q", "-b", "other")
	head := gitRun(t, dir, "rev-parse", "HEAD")

	committed, err := ChangeStatus(dir, "main", "prd/a/x.md", choice(t, StatusImplemented), changeDay)
	if err == nil || committed || !strings.Contains(err.Error(), "other") {
		t.Fatalf("ChangeStatus = %v, %v", committed, err)
	}
	if got := readFile(t, filepath.Join(dir, "prd/a/x.md")); got != changePRD {
		t.Errorf("file changed: %q", got)
	}
	if gitRun(t, dir, "rev-parse", "HEAD") != head {
		t.Error("new commit")
	}
}

func TestChangeStatusUncommittedChanges(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, dir string){
		"unstaged": func(t *testing.T, dir string) {},
		"staged": func(t *testing.T, dir string) {
			gitRun(t, dir, "add", "prd/a/x.md")
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "prd/a/x.md", changePRD)
			commitAll(t, dir, "init")
			edited := changePRD + "More\n"
			writeFile(t, dir, "prd/a/x.md", edited)
			prepare(t, dir)

			committed, err := ChangeStatus(dir, "main", "prd/a/x.md", choice(t, StatusImplemented), changeDay)
			if err == nil || committed || !strings.Contains(err.Error(), "uncommitted") {
				t.Fatalf("ChangeStatus = %v, %v", committed, err)
			}
			if got := readFile(t, filepath.Join(dir, "prd/a/x.md")); got != edited {
				t.Errorf("file changed: %q", got)
			}
		})
	}
}

func TestChangeStatusNoStatusLine(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "prd/a/x.md", "# Notes\n")
	commitAll(t, dir, "init")
	head := gitRun(t, dir, "rev-parse", "HEAD")
	committed, err := ChangeStatus(dir, "main", "prd/a/x.md", choice(t, StatusImplemented), changeDay)
	if err == nil || committed {
		t.Fatalf("ChangeStatus = %v, %v", committed, err)
	}
	if gitRun(t, dir, "rev-parse", "HEAD") != head {
		t.Error("new commit")
	}
}

func TestChangeStatusMissingFile(t *testing.T) {
	dir := initRepo(t)
	if _, err := ChangeStatus(dir, "main", "prd/a/none.md", choice(t, StatusPlanned), changeDay); err == nil {
		t.Error("no error for a missing file")
	}
}
