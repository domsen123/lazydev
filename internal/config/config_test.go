package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxParallel != DefaultMaxParallel || cfg.PortBase != DefaultPortBase || cfg.Claude.Binary != DefaultClaudeBinary {
		t.Errorf("Standardwerte fehlen: %+v", cfg)
	}
}

func TestLoadSpecExample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, `maxParallel: 2
portBase: 3200
claude:
  binary: claude
projects:
  - path: ~/development/nuxt/nuxt-drizzle
    preset: nuxt-drizzle
    autoMerge: true
  - path: ~/development/nuxt/shop
    preset: nuxt-drizzle
    autoMerge: false
    allowedTools:
      - "Bash(pnpm test:e2e:*)"
    hooks:
      setup: ./scripts/lazydev-setup.sh
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxParallel != 2 || cfg.PortBase != 3200 || len(cfg.Projects) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
	b := cfg.Projects[1]
	if b.AutoMergeEnabled() || b.Hooks.Setup != "./scripts/lazydev-setup.sh" || b.AllowedTools[0] != "Bash(pnpm test:e2e:*)" {
		t.Errorf("shop = %+v", b)
	}
	if !cfg.Projects[0].AutoMergeEnabled() {
		t.Error("autoMerge: true nicht übernommen")
	}
	home, _ := os.UserHomeDir()
	if got := cfg.Projects[0].AbsPath(); got != filepath.Join(home, "development/nuxt/nuxt-drizzle") {
		t.Errorf("AbsPath = %q", got)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	for name, content := range map[string]string{
		"unbekanntes Preset": "projects:\n  - path: /x\n    preset: rails\n",
		"doppelt":            "projects:\n  - path: /x\n  - path: /x/\n",
		"ohne Pfad":          "projects:\n  - preset: nuxt-drizzle\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			writeFile(t, path, content)
			if _, err := Load(path); err == nil {
				t.Error("Fehler erwartet")
			}
		})
	}
}

func TestAddRemoveSaveRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "app")
	writeFile(t, filepath.Join(proj, "drizzle.config.ts"), "")
	if err := os.MkdirAll(filepath.Join(proj, "server/database/migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(tmp, "plain")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, _ := Load(filepath.Join(tmp, "none.yaml"))
	p, err := cfg.AddProject(proj)
	if err != nil {
		t.Fatal(err)
	}
	if p.Preset != PresetNuxtDrizzle {
		t.Errorf("Preset = %q", p.Preset)
	}
	if _, err := cfg.AddProject(proj + "/"); err == nil || !strings.Contains(err.Error(), "schon eingetragen") {
		t.Errorf("doppeltes Hinzufügen: %v", err)
	}
	if _, err := cfg.AddProject(filepath.Join(tmp, "fehlt")); err == nil {
		t.Error("fehlender Ordner akzeptiert")
	}
	if _, err := cfg.AddProject(other); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmp, "cfg", "config.yaml")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Projects) != 2 || loaded.Projects[1].Preset != "" {
		t.Fatalf("geladen: %+v", loaded.Projects)
	}

	if err := loaded.RemoveProject(0); err != nil {
		t.Fatal(err)
	}
	if err := loaded.RemoveProject(5); err == nil {
		t.Error("ungültiger Index akzeptiert")
	}
	if len(loaded.Projects) != 1 || loaded.Projects[0].AbsPath() != other {
		t.Errorf("nach Entfernen: %+v", loaded.Projects)
	}
	if _, err := os.Stat(proj); err != nil {
		t.Error("RemoveProject hat Dateien gelöscht")
	}
}

func TestContractExpand(t *testing.T) {
	home, _ := os.UserHomeDir()
	abs := filepath.Join(home, "dev", "x")
	if got := ContractPath(abs); got != "~/dev/x" {
		t.Errorf("ContractPath = %q", got)
	}
	if got := ExpandPath("~/dev/x"); got != abs {
		t.Errorf("ExpandPath = %q", got)
	}
	if got := ContractPath("/opt/x"); got != "/opt/x" {
		t.Errorf("ContractPath außerhalb von home = %q", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
