// Package skill holds the Claude Code skill "lazydev" and installs it into the user's skills
// folder. The skill tells a normal Claude Code session what lazydev is, how PRDs work and that
// "lazydev prompt create" prints the create prompt of a project. Its files are embedded, so the
// skill belongs to the version of the binary, not to a repository.
package skill

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"lazydev/internal/fsutil"
)

// files/references/spec.md is a copy of SPEC.md: go:embed cannot read outside the package folder.
// TestSpecCopy fails when the copy is out of date.
//
//go:generate cp ../../SPEC.md files/references/spec.md
//go:embed files
var files embed.FS

// Name is the name of the skill and of its folder under skills/.
const Name = "lazydev"

// Dir is the folder of the skill: $CLAUDE_CONFIG_DIR/skills/lazydev, without the variable
// $HOME/.claude/skills/lazydev.
func Dir() string {
	root := os.Getenv("CLAUDE_CONFIG_DIR")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".claude")
	}
	return filepath.Join(root, "skills", Name)
}

// Install writes the skill into Dir. Every embedded file is written only when it is missing or
// differs, atomically. Files in the folder that the binary does not have get deleted, empty
// subfolders after them. changed is true when something was written or deleted.
func Install() (changed bool, err error) {
	dir := Dir()
	keep := map[string]bool{}
	err = fs.WalkDir(files, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("files", path)
		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		keep[rel] = true
		want, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		if have, err := os.ReadFile(target); err == nil && bytes.Equal(have, want) {
			return nil
		}
		changed = true
		return fsutil.WriteAtomic(target, want, 0o644)
	})
	if err != nil {
		return changed, err
	}
	// Deleting happens under dir only, and Dir always ends in skills/lazydev, so nothing outside
	// the skill folder can go.
	var dirs []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if keep[rel] {
			return nil
		}
		changed = true
		return os.Remove(path)
	})
	if err != nil {
		return changed, err
	}
	// Deepest folders first, so a folder whose only content was an empty folder goes too.
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := os.ReadDir(dirs[i])
		if err != nil {
			return changed, err
		}
		if len(entries) > 0 {
			continue
		}
		if err := os.Remove(dirs[i]); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}
