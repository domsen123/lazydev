package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lazydev/internal/config"
)

// TestInstallSkillOff checks that skill: false writes no skill but still updates an installed mod.
func TestInstallSkillOff(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	stale := filepath.Join(root, "skills", "lazydev-runs", "hooks", "old.tsx")
	os.MkdirAll(filepath.Dir(stale), 0o755)
	os.WriteFile(stale, []byte("x"), 0o644)
	off := false
	if msg := installSkill(config.Settings{Skill: &off})(); msg != nil {
		t.Errorf("msg: %#v", msg)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "lazydev")); err == nil {
		t.Error("skill written with skill: false")
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "lazydev-runs", "hooks", "register.tsx")); err != nil {
		t.Errorf("installed mod not updated: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("stale mod file kept")
	}
}

func TestInstallSkillLeavesModOut(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	installSkill(config.Settings{})()
	if _, err := os.Stat(filepath.Join(root, "skills", "lazydev-runs")); err == nil {
		t.Error("the TUI installed the mod by itself")
	}
}

func TestInstallSkillWrites(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	cmd := installSkill(config.Settings{})
	if cmd == nil {
		t.Fatal("installSkill returned nil")
	}
	if msg := cmd(); msg != nil {
		t.Errorf("msg: %#v", msg)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "lazydev", "SKILL.md")); err != nil {
		t.Error(err)
	}
}

func TestSkillMsgSetsNote(t *testing.T) {
	next, _ := cockpitModel(t).Update(skillMsg{err: errors.New("boom")})
	if m := next.(model); m.note != "skill install failed: boom" {
		t.Errorf("note: %q", m.note)
	}
}

func TestSkillMsgKeepsErr(t *testing.T) {
	m := cockpitModel(t)
	m.err = "other"
	next, _ := m.Update(skillMsg{err: errors.New("boom")})
	if m := next.(model); m.note != "" || m.err != "other" {
		t.Errorf("note %q, err %q", m.note, m.err)
	}
}
