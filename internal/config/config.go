// Package config liest und schreibt ~/.config/lazydev/config.yaml.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultMaxParallel  = 3
	DefaultPortBase     = 3100
	DefaultClaudeBinary = "claude"
	// DefaultBaseBranch gilt, wenn ein Projekt kein baseBranch setzt.
	DefaultBaseBranch = "main"
)

type Config struct {
	MaxParallel int       `yaml:"maxParallel"`
	PortBase    int       `yaml:"portBase"`
	Claude      Claude    `yaml:"claude"`
	Projects    []Project `yaml:"projects"`
}

type Claude struct {
	Binary string `yaml:"binary"`
}

type Project struct {
	// Path steht so in der Datei, wie der Mensch ihn schreibt, also gern mit "~".
	Path   string `yaml:"path"`
	Preset string `yaml:"preset,omitempty"`
	// BaseBranch ist der Ziel-Branch: Von ihm zweigen Worktrees ab, in ihn mergt lazydev.
	BaseBranch   string   `yaml:"baseBranch,omitempty"`
	AutoMerge    *bool    `yaml:"autoMerge,omitempty"`
	AllowedTools []string `yaml:"allowedTools,omitempty"`
	Hooks        Hooks    `yaml:"hooks,omitempty"`
}

// Hooks überschreibt einzelne Hooks des Presets.
type Hooks struct {
	Setup    string `yaml:"setup,omitempty"`
	Teardown string `yaml:"teardown,omitempty"`
	// Check führt die Prüfbefehle vor dem Merge aus. Exit-Code ungleich 0 heißt: rot.
	Check string `yaml:"check,omitempty"`
	// Dev startet den Dev-Server im Worktree, mit PORT aus der .env des Worktrees.
	Dev string `yaml:"dev,omitempty"`
}

// AbsPath liefert den Projektpfad absolut und mit aufgelöstem "~".
func (p Project) AbsPath() string {
	return ExpandPath(p.Path)
}

// Base liefert BaseBranch, ohne Angabe DefaultBaseBranch.
func (p Project) Base() string {
	if p.BaseBranch == "" {
		return DefaultBaseBranch
	}
	return p.BaseBranch
}

// AutoMergeEnabled liefert den Standard true, wenn autoMerge fehlt.
func (p Project) AutoMergeEnabled() bool {
	return p.AutoMerge == nil || *p.AutoMerge
}

// DefaultPath beachtet LAZYDEV_CONFIG und XDG_CONFIG_HOME.
func DefaultPath() string {
	if p := os.Getenv("LAZYDEV_CONFIG"); p != "" {
		return ExpandPath(p)
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(base, "lazydev", "config.yaml")
}

// Load liest die Konfiguration. Fehlt die Datei, liefert Load die Standardwerte.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Save schreibt atomar über eine temporäre Datei im selben Ordner.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// AddProject trägt einen bestehenden Ordner ein und erkennt das Preset.
func (c *Config) AddProject(path string) (Project, error) {
	abs := ExpandPath(path)
	info, err := os.Stat(abs)
	if err != nil {
		return Project{}, err
	}
	if !info.IsDir() {
		return Project{}, fmt.Errorf("%s ist kein Ordner", abs)
	}
	for _, p := range c.Projects {
		if p.AbsPath() == abs {
			return Project{}, fmt.Errorf("%s ist schon eingetragen", ContractPath(abs))
		}
	}
	p := Project{Path: ContractPath(abs), Preset: DetectPreset(abs)}
	c.Projects = append(c.Projects, p)
	return p, nil
}

// RemoveProject entfernt den Eintrag, löscht aber keine Dateien.
func (c *Config) RemoveProject(i int) error {
	if i < 0 || i >= len(c.Projects) {
		return fmt.Errorf("kein Projekt an Position %d", i)
	}
	c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
	return nil
}

func (c *Config) applyDefaults() {
	if c.MaxParallel <= 0 {
		c.MaxParallel = DefaultMaxParallel
	}
	if c.PortBase <= 0 {
		c.PortBase = DefaultPortBase
	}
	if c.Claude.Binary == "" {
		c.Claude.Binary = DefaultClaudeBinary
	}
}

func (c *Config) validate() error {
	seen := map[string]bool{}
	for _, p := range c.Projects {
		if p.Path == "" {
			return errors.New("Projekt ohne path")
		}
		if p.Preset != "" && !IsKnownPreset(p.Preset) {
			return fmt.Errorf("%s: unbekanntes Preset %q", p.Path, p.Preset)
		}
		abs := p.AbsPath()
		if seen[abs] {
			return fmt.Errorf("%s ist doppelt eingetragen", p.Path)
		}
		seen[abs] = true
	}
	return nil
}

// ExpandPath löst "~" auf und macht den Pfad absolut.
func ExpandPath(p string) string {
	if p == "~" {
		p = homeDir()
	} else if strings.HasPrefix(p, "~/") {
		p = filepath.Join(homeDir(), p[2:])
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

// ContractPath ersetzt das Home-Verzeichnis durch "~".
func ContractPath(abs string) string {
	home := homeDir()
	if abs == home {
		return "~"
	}
	if rel, ok := strings.CutPrefix(abs, home+string(filepath.Separator)); ok {
		return "~/" + rel
	}
	return abs
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return home
}
