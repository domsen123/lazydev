// Package config reads and writes lazydev's own files under $XDG_CONFIG_HOME/lazydev/: the project
// list and the settings. Settings of a project live in its .lazydev/, see package repo.
package config

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"lazydev/internal/fsutil"
)

// Dir is lazydev's config folder.
func Dir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lazydev")
}

func projectsPath() string { return filepath.Join(Dir(), "projects.json") }
func settingsPath() string { return filepath.Join(Dir(), "settings.json") }

// storedProjects is the file format of projects.json.
type storedProjects struct {
	Projects []string `json:"projects"`
}

// LoadProjects reads the paths of the project list. A missing file means no projects.
func LoadProjects() ([]string, error) {
	data, err := os.ReadFile(projectsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stored storedProjects
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("%s: %w", projectsPath(), err)
	}
	return stored.Projects, nil
}

// SaveProjects writes the project list.
func SaveProjects(paths []string) error {
	data, err := json.MarshalIndent(storedProjects{Projects: paths}, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(projectsPath(), append(data, '\n'), 0o644)
}

// Settings are the settings of the TUI and of the Claude steps of the runner.
type Settings struct {
	// Theme is the name of the color theme.
	Theme string `json:"theme,omitempty"`
	// AllowedTools add to claude.DefaultAllowedTools in every Claude step of all projects. They
	// belong to the user and need no trust.
	AllowedTools []string `json:"allowedTools,omitempty"`
	// PermissionMode is the --permission-mode of every Claude step, see Mode.
	PermissionMode string `json:"permissionMode,omitempty"`
	// Discord holds the webhook of the runner's notifications.
	Discord Discord `json:"discord,omitzero"`
	// Skill nil and true let the TUI write the Claude Code skill lazydev at its start, false
	// switches that off. lazydev skill install works either way.
	Skill *bool `json:"skill,omitempty"`
}

// SkillEnabled reports whether the TUI writes the Claude Code skill at its start.
func (s Settings) SkillEnabled() bool {
	return s.Skill == nil || *s.Skill
}

// Discord is the webhook runners post their notifications to, global and per project.
type Discord struct {
	// Enabled is the global switch, off by default.
	Enabled bool   `json:"enabled,omitempty"`
	URL     string `json:"url,omitempty"`
	// Projects holds the settings of single projects by their absolute path.
	Projects map[string]DiscordProject `json:"projects,omitempty"`
}

// DiscordProject is the Discord setting of one project.
type DiscordProject struct {
	// Enabled nil counts as on, false mutes the project.
	Enabled *bool `json:"enabled,omitempty"`
	// URL replaces the global URL, empty uses it.
	URL string `json:"url,omitempty"`
}

// ProjectEnabled reports whether the project in dir is switched on. Without entry or field it is.
func (d Discord) ProjectEnabled(dir string) bool {
	p, ok := d.Projects[dir]
	if !ok || p.Enabled == nil {
		return true
	}
	return *p.Enabled
}

// ProjectURL is the URL of the project in dir, the global URL if it has none.
func (d Discord) ProjectURL(dir string) string {
	if u := d.Projects[dir].URL; u != "" {
		return u
	}
	return d.URL
}

// Target is the URL notifications of the project in dir go to, empty if a switch is off.
func (d Discord) Target(dir string) string {
	if !d.Enabled || !d.ProjectEnabled(dir) {
		return ""
	}
	return d.ProjectURL(dir)
}

// Permission modes a Claude step may run with.
const (
	ModeAcceptEdits = "acceptEdits"
	ModeAuto        = "auto"
)

// Mode is the permission mode of the Claude steps: acceptEdits for an empty value, the value for
// acceptEdits and auto, an error otherwise.
func (s Settings) Mode() (string, error) {
	switch s.PermissionMode {
	case "":
		return ModeAcceptEdits, nil
	case ModeAcceptEdits, ModeAuto:
		return s.PermissionMode, nil
	}
	return "", fmt.Errorf("settings.json: unknown permissionMode %q, allowed: %s, %s", s.PermissionMode, ModeAcceptEdits, ModeAuto)
}

// LoadSettings reads settings.json. A missing or broken file gives the defaults.
func LoadSettings() Settings {
	var s Settings
	if data, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(data, &s)
	}
	return s
}

// SaveSettings writes settings.json, readable by the user only: it may hold webhook URLs.
func SaveSettings(s Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(settingsPath(), append(data, '\n'), 0o600)
}

// IDs assigns the project IDs that name state and worktree folders: the folder name, plus a short
// hash of the path for folders that share a name. So the ID of a project does not depend on the
// order of the list.
func IDs(paths []string) []string {
	count := map[string]int{}
	for _, p := range paths {
		count[filepath.Base(p)]++
	}
	ids := make([]string, len(paths))
	for i, p := range paths {
		ids[i] = filepath.Base(p)
		if count[ids[i]] > 1 {
			sum := sha1.Sum([]byte(p))
			ids[i] += "-" + hex.EncodeToString(sum[:])[:6]
		}
	}
	return ids
}

// ExpandPath resolves "~" and makes the path absolute and clean.
func ExpandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return filepath.Abs(path)
}

// DisplayPath shortens the home directory to "~".
func DisplayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + rest
	}
	return path
}

// CompletePath completes the last part of input to the matching folders. One match gets a
// trailing slash, several matches complete to their common prefix. Hidden folders only match when
// the typed part starts with a dot.
func CompletePath(input string) string {
	dir, base := filepath.Split(input)
	absDir, err := ExpandPath(dir)
	if dir == "" || err != nil {
		return input
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return input
	}
	var matches []string
	for _, e := range entries {
		name := e.Name()
		if !fsutil.IsDir(filepath.Join(absDir, name)) || !strings.HasPrefix(name, base) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		matches = append(matches, name)
	}
	switch len(matches) {
	case 0:
		return input
	case 1:
		return dir + matches[0] + "/"
	}
	prefix := matches[0]
	for _, m := range matches[1:] {
		// Cut whole characters, so the prefix stays valid UTF-8.
		for !strings.HasPrefix(m, prefix) {
			_, size := utf8.DecodeLastRuneInString(prefix)
			prefix = prefix[:len(prefix)-size]
		}
	}
	return dir + prefix
}
