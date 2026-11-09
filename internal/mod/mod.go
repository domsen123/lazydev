// Package mod holds the Claude Code mod "lazydev-runs", a pane with the PRD runs of lazydev, and
// installs it into the user's skills folder, where Claude Code loads a folder with
// .claude-plugin/plugin.json as a plugin. Its files are embedded, so the mod belongs to the
// version of the binary. Unlike the skill it is opt-in: only "lazydev mod install" adds it, the
// TUI keeps an installed one current.
package mod

import (
	"embed"
	"os"

	"lazydev/internal/fsutil"
	"lazydev/internal/skill"
)

// all: keeps .claude-plugin, which go:embed leaves out otherwise.
//
//go:embed all:files
var files embed.FS

// Name is the name of the mod and of its folder under skills/.
const Name = "lazydev-runs"

// Dir is the folder of the mod: $CLAUDE_CONFIG_DIR/skills/lazydev-runs, without the variable
// $HOME/.claude/skills/lazydev-runs.
func Dir() string {
	return skill.SkillsDir(Name)
}

// Installed reports whether the mod's folder exists.
func Installed() bool {
	return fsutil.IsDir(Dir())
}

// Install writes the mod into Dir like skill.Install: changed files only, files the binary does
// not have get deleted.
func Install() (changed bool, err error) {
	return skill.Sync(files, "files", Dir())
}

// Update rewrites an installed mod and leaves a missing one missing.
func Update() (changed bool, err error) {
	if !Installed() {
		return false, nil
	}
	return Install()
}

// Uninstall removes the mod's folder. A missing folder is no error.
func Uninstall() error {
	return os.RemoveAll(Dir())
}
