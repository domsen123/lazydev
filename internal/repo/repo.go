// Package repo reads and writes .lazydev/ in the main folder of a project: config.yaml and the
// prompts of the steps. The folder is versioned with the project, so everyone in the team
// works the same way. Adding a project creates it with the prompts of the simple workflow, and the
// workflow reads prompts from there only.
package repo

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"lazydev/internal/state"
)

// Dir is lazydev's folder in a project.
const Dir = ".lazydev"

// DefaultBaseBranch applies without baseBranch in config.yaml.
const DefaultBaseBranch = "main"

// Steps are the steps with a prompt of their own, in order. unblock is the dialog of f that
// resolves a blocked run, no runner step.
var Steps = []string{"create", "implement", "review", "merge", "unblock"}

// KnowledgeAll is the knowledge key for files every step reads.
const KnowledgeAll = "all"

// KnowledgeKeys are the allowed keys under knowledge, in the order config.yaml lists them.
var KnowledgeKeys = append([]string{KnowledgeAll}, Steps...)

// workflows/simple/ holds the prompts Init copies into projects. They started as the "einfach"
// workflow of the old lazydev.
//
//go:embed workflows/simple/*.md
var simpleWorkflow embed.FS

// Hooks are scripts the runner starts in the worktree. Relative paths start at the main folder.
type Hooks struct {
	Setup    string `yaml:"setup,omitempty"`
	Teardown string `yaml:"teardown,omitempty"`
	// Check runs before the merge. An exit code other than 0 means red.
	Check string `yaml:"check,omitempty"`
	// Dev starts the dev server of a worktree.
	Dev string `yaml:"dev,omitempty"`
}

// Config is .lazydev/config.yaml. Unknown fields, like the preset of the old lazydev, are ignored.
type Config struct {
	BaseBranch string `yaml:"baseBranch,omitempty"`
	// Knowledge lists per step paths relative to the project that Claude reads first.
	Knowledge    map[string][]string `yaml:"knowledge,omitempty"`
	AllowedTools []string            `yaml:"allowedTools,omitempty"`
	Hooks        Hooks               `yaml:"hooks,omitempty"`
}

// Base is BaseBranch, DefaultBaseBranch without one.
func (c Config) Base() string {
	if c.BaseBranch == "" {
		return DefaultBaseBranch
	}
	return c.BaseBranch
}

// trusted are the sections that run code or widen Claude's rights.
type trusted struct {
	AllowedTools []string `yaml:"allowedTools,omitempty"`
	Hooks        Hooks    `yaml:"hooks,omitempty"`
}

// TrustSections are allowedTools and hooks as YAML, empty without both.
func (c Config) TrustSections() string {
	if len(c.AllowedTools) == 0 && c.Hooks == (Hooks{}) {
		return ""
	}
	data, _ := yaml.Marshal(trusted{AllowedTools: c.AllowedTools, Hooks: c.Hooks})
	return string(data)
}

// TrustHash is the SHA-256 over TrustSections. Comments and other fields do not change it.
func (c Config) TrustHash() string {
	s := c.TrustSections()
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TrustState says whether the runner may start hooks and allowed tools of config.yaml.
type TrustState int

const (
	// TrustNone: no hooks and no allowed tools, nothing to confirm.
	TrustNone TrustState = iota
	TrustConfirmed
	// TrustPending: the runner refuses to start until the user trusts the config.
	TrustPending
)

// Info is the lazydev setup of a project.
type Info struct {
	// Initialized is true if .lazydev/config.yaml exists.
	Initialized bool
	Err         error
	Config      Config
	// Prompts is true per step whose .lazydev/<step>.md exists.
	Prompts []bool
	Trust   TrustState
	// ID is the project ID, which names the trust file.
	ID string
}

// Read reads the lazydev setup of the project at dir with ID id.
func Read(dir, id string) Info {
	info := Info{ID: id}
	for _, step := range Steps {
		_, err := os.Stat(PromptPath(dir, step))
		info.Prompts = append(info.Prompts, err == nil)
	}
	data, err := os.ReadFile(ConfigPath(dir))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return info
	case err != nil:
		info.Err = err
		return info
	}
	info.Initialized = true
	if err := yaml.Unmarshal(data, &info.Config); err != nil {
		info.Err = fmt.Errorf("config.yaml: %w", err)
		return info
	}
	if hash := info.Config.TrustHash(); hash != "" {
		info.Trust = TrustPending
		if state.Trusted(id, hash) {
			info.Trust = TrustConfirmed
		}
	}
	return info
}

// ConfigPath is .lazydev/config.yaml of the project at dir.
func ConfigPath(dir string) string {
	return filepath.Join(dir, Dir, "config.yaml")
}

// PromptPath is .lazydev/<step>.md of the project at dir.
func PromptPath(dir, step string) string {
	return filepath.Join(dir, Dir, step+".md")
}

// Init sets up .lazydev/ in the project at dir: config.yaml with the base branch, and the prompts
// of the simple workflow. It only adds missing files, so a project keeps its own config and
// prompts, and a deleted file comes back.
func Init(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, Dir), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(ConfigPath(dir)); err != nil {
		// The default main stays out, and an empty config is an empty file.
		var data []byte
		if base := DetectBaseBranch(dir); base != "" && base != DefaultBaseBranch {
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(Config{BaseBranch: base}); err != nil {
				return err
			}
			enc.Close()
			data = buf.Bytes()
		}
		if err := os.WriteFile(ConfigPath(dir), data, 0o644); err != nil {
			return err
		}
	}
	for _, step := range Steps {
		if _, err := os.Stat(PromptPath(dir, step)); err == nil {
			continue
		}
		if err := WritePrompt(dir, step); err != nil {
			return err
		}
	}
	return nil
}

// WritePrompt writes the prompt of the simple workflow for step into .lazydev/, overwriting it.
func WritePrompt(dir, step string) error {
	data, err := simpleWorkflow.ReadFile("workflows/simple/" + step + ".md")
	if err != nil {
		return err
	}
	return os.WriteFile(PromptPath(dir, step), data, 0o644)
}
