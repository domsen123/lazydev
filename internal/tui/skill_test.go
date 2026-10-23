package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lazydev/internal/config"
)

func TestInstallSkillOff(t *testing.T) {
	off := false
	if installSkill(config.Settings{Skill: &off}) != nil {
		t.Error("installSkill runs with skill: false")
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
