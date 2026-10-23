package skill

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	dir := filepath.Join(root, "skills", "lazydev")
	if Dir() != dir {
		t.Fatalf("Dir: %q", Dir())
	}

	changed, err := Install()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("first install reports no change")
	}
	skill, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := files.ReadFile("files/SKILL.md")
	if !bytes.Equal(skill, want) {
		t.Error("SKILL.md differs from the embedded file")
	}
	if info, err := os.Stat(filepath.Join(dir, "references", "spec.md")); err != nil {
		t.Fatal(err)
	} else if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("perm: %v", perm)
	}

	changed, err = Install()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("second install reports a change")
	}

	// A changed file comes back, a foreign file goes.
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("edited"), 0o644)
	alt := filepath.Join(dir, "references", "alt.md")
	os.WriteFile(alt, []byte("foreign"), 0o644)
	changed, err = Install()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("reset reports no change")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "SKILL.md")); !bytes.Equal(got, want) {
		t.Errorf("SKILL.md not reset: %q", got)
	}
	if _, err := os.Stat(alt); err == nil {
		t.Error("foreign file survived")
	}
}

func TestInstallRemovesEmptyFolders(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if _, err := Install(); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(Dir(), "old", "deeper")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "x.md"), []byte("x"), 0o644)
	changed, err := Install()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("cleanup reports no change")
	}
	if _, err := os.Stat(filepath.Join(Dir(), "old")); err == nil {
		t.Error("empty folder survived")
	}
	if _, err := os.Stat(filepath.Join(Dir(), "references", "spec.md")); err != nil {
		t.Error("references/spec.md is gone")
	}
}

func TestDirDefault(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", "/home/u")
	if got := Dir(); !strings.HasSuffix(got, filepath.Join(".claude", "skills", "lazydev")) || !strings.HasPrefix(got, "/home/u/") {
		t.Errorf("Dir: %q", got)
	}
}

func TestSpecCopy(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "..", "SPEC.md"))
	if err != nil {
		t.Fatal(err)
	}
	copied, err := files.ReadFile("files/references/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(spec, copied) {
		t.Fatal("references/spec.md differs from SPEC.md, run go generate ./internal/skill")
	}
}

func TestSkillFrontmatter(t *testing.T) {
	data, err := files.ReadFile("files/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		t.Error("SKILL.md does not start with frontmatter")
	}
	for _, want := range []string{"name: lazydev", "description:"} {
		if !strings.Contains(s, want) {
			t.Errorf("SKILL.md lacks %q", want)
		}
	}
	if n := strings.Count(s, "\n"); n >= 80 {
		t.Errorf("SKILL.md has %d lines, want fewer than 80", n)
	}
}
