package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fullRepo = `# Einstellungen für lazydev
baseBranch: next
preset: nuxt-drizzle
knowledge:
  all: [CLAUDE.md]
  implement: [docs/db.md]
allowedTools:
  - "Bash(pnpm test:e2e:*)"
hooks:
  setup: ./scripts/setup.sh
  dev: ./scripts/dev.sh
`

func TestLoadRepoWithoutFile(t *testing.T) {
	r, exists, err := LoadRepo(t.TempDir())
	if err != nil || exists {
		t.Fatalf("ohne Datei: %v, %v", exists, err)
	}
	if r.Base() != DefaultBaseBranch || r.TrustHash() != "" {
		t.Errorf("Standardwerte: %+v", r)
	}
}

func TestLoadRepoAllFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, RepoPath(dir), fullRepo)
	r, exists, err := LoadRepo(dir)
	if err != nil || !exists {
		t.Fatalf("LoadRepo: %v, %v", exists, err)
	}
	if r.Base() != "next" || r.Preset != PresetNuxtDrizzle || r.AllowedTools[0] != "Bash(pnpm test:e2e:*)" ||
		r.Hooks.Setup != "./scripts/setup.sh" || r.Hooks.Dev != "./scripts/dev.sh" ||
		!slices.Equal(r.Knowledge["all"], []string{"CLAUDE.md"}) || !slices.Equal(r.Knowledge["implement"], []string{"docs/db.md"}) {
		t.Errorf("Repo: %+v", r)
	}
	// Eine leere Datei gilt als vorhanden, mit Standardwerten.
	writeFile(t, RepoPath(dir), "")
	if r, exists, err := LoadRepo(dir); err != nil || !exists || r.Base() != DefaultBaseBranch {
		t.Errorf("leere Datei: %+v, %v, %v", r, exists, err)
	}
}

func TestLoadRepoRejectsUnknown(t *testing.T) {
	for content, want := range map[string]string{
		"baseBranch: next\nautoMerge: true\n": `unbekanntes Feld "autoMerge"`,
		"hooks:\n  start: ./x.sh\n":           `unbekanntes Feld "hooks.start"`,
		"knowledge:\n  deploy: [CLAUDE.md]\n": `unbekannter Schritt "deploy" unter knowledge`,
		"preset: rails\n":                     `unbekanntes Preset "rails"`,
		"allowedTools: Bash\n":                "config.yaml",
	} {
		dir := t.TempDir()
		writeFile(t, RepoPath(dir), content)
		_, _, err := LoadRepo(dir)
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), RepoPath(dir)) {
			t.Errorf("%q: Fehler %v, erwartet %q mit Dateipfad", content, err, want)
		}
	}
}

func TestEffective(t *testing.T) {
	dir := t.TempDir()
	local := Project{Path: dir, BaseBranch: "dev", Preset: PresetNuxtDrizzle, AllowedTools: []string{"Bash(x:*)"},
		Hooks: Hooks{Check: "./check.sh"}}

	r, ignored, err := Effective(local)
	if err != nil || len(ignored) != 0 {
		t.Fatalf("ohne .lazydev: %v, %v", ignored, err)
	}
	if r.Base() != "dev" || r.Preset != PresetNuxtDrizzle || r.AllowedTools[0] != "Bash(x:*)" || r.Hooks.Check != "./check.sh" {
		t.Errorf("Übergangsfelder: %+v", r)
	}

	writeFile(t, RepoPath(dir), "baseBranch: next\n")
	r, ignored, err = Effective(local)
	if err != nil {
		t.Fatal(err)
	}
	if r.Base() != "next" || r.Preset != "" || len(r.AllowedTools) != 0 || r.Hooks != (Hooks{}) {
		t.Errorf("mit .lazydev wirken lokale Werte: %+v", r)
	}
	if strings.Join(ignored, ",") != "baseBranch,preset,allowedTools,hooks" {
		t.Errorf("ignoriert: %v", ignored)
	}
	if _, ignored, _ := Effective(Project{Path: dir}); len(ignored) != 0 {
		t.Errorf("ohne lokale Werte ignoriert: %v", ignored)
	}
}

func TestSetRepoBaseBranchKeepsComments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, RepoPath(dir), fullRepo)
	if err := SetRepoBaseBranch(dir, "release"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(RepoPath(dir))
	if !strings.Contains(string(data), "# Einstellungen für lazydev") || !strings.Contains(string(data), "baseBranch: release") ||
		strings.Index(string(data), "baseBranch") > strings.Index(string(data), "preset") {
		t.Errorf("nach release:\n%s", data)
	}
	if r, _, err := LoadRepo(dir); err != nil || r.Base() != "release" || r.Hooks.Setup != "./scripts/setup.sh" {
		t.Errorf("geladen: %+v, %v", r, err)
	}

	// main ist der Standard und steht nicht in der Datei.
	if err := SetRepoBaseBranch(dir, DefaultBaseBranch); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(RepoPath(dir))
	if strings.Contains(string(data), "baseBranch") || !strings.Contains(string(data), "# Einstellungen für lazydev") ||
		!strings.Contains(string(data), "preset: nuxt-drizzle") {
		t.Errorf("nach main:\n%s", data)
	}

	// Ohne baseBranch kommt das Feld an den Anfang.
	writeFile(t, RepoPath(dir), "preset: nuxt-drizzle # Kommentar\n")
	if err := SetRepoBaseBranch(dir, "next"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(RepoPath(dir))
	if string(data) != "baseBranch: next\npreset: nuxt-drizzle # Kommentar\n" {
		t.Errorf("neues Feld:\n%s", data)
	}
	if err := SetRepoBaseBranch(filepath.Join(dir, "fehlt"), "next"); err == nil {
		t.Error("ohne Datei kein Fehler")
	}
}

func TestTrustHash(t *testing.T) {
	a := Repo{AllowedTools: []string{"Bash(x:*)"}, Hooks: Hooks{Setup: "./s.sh"}}
	// Andere Felder ändern den Hash nicht.
	b := a
	b.BaseBranch, b.Preset, b.Knowledge = "next", PresetNuxtDrizzle, map[string][]string{"all": {"CLAUDE.md"}}
	if a.TrustHash() == "" || a.TrustHash() != b.TrustHash() || len(a.TrustHash()) != 64 {
		t.Errorf("Hash: %q, %q", a.TrustHash(), b.TrustHash())
	}
	c := a
	c.Hooks.Setup = "./anders.sh"
	if c.TrustHash() == a.TrustHash() {
		t.Error("geänderter Hook ändert den Hash nicht")
	}
	if (Repo{BaseBranch: "next"}).TrustHash() != "" {
		t.Error("ohne Hooks und Freigaben nicht leer")
	}
	// Kommentare in der Datei ändern den Hash nicht.
	dir := t.TempDir()
	writeFile(t, RepoPath(dir), fullRepo)
	r1, _, _ := LoadRepo(dir)
	writeFile(t, RepoPath(dir), strings.ReplaceAll(fullRepo, "hooks:", "# Hooks des Teams\nhooks:"))
	r2, _, _ := LoadRepo(dir)
	if r1.TrustHash() != r2.TrustHash() {
		t.Error("Kommentar ändert den Hash")
	}
}

func TestRemoveLocalProjectSettings(t *testing.T) {
	on := true
	cfg := &Config{Projects: []Project{{Path: "/x", AutoMerge: &on, BaseBranch: "next", Preset: PresetNuxtDrizzle,
		AllowedTools: []string{"a"}, Hooks: Hooks{Dev: "d"}}}}
	cfg.RemoveLocalProjectSettings(0)
	cfg.RemoveLocalProjectSettings(3)
	p := cfg.Projects[0]
	if p.Path != "/x" || p.AutoMerge == nil || p.BaseBranch != "" || p.Preset != "" || p.AllowedTools != nil || p.Hooks != (Hooks{}) {
		t.Errorf("nach dem Entfernen: %+v", p)
	}
}
