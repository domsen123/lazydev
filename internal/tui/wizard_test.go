package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/docker"
	"lazydev/internal/repo"
)

// fakeComposeDocker answers "compose config" with $FAKE_COMPOSE and fails for broken.yml.
const fakeComposeDocker = `#!/bin/sh
case "$*" in
*broken.yml*) echo "yaml: line 3: mapping values are not allowed" >&2; echo "second" >&2; exit 15 ;;
"compose config --format json"|"compose -f "*" config --format json") printf '%s' "$FAKE_COMPOSE" ;;
esac
`

const composeJSON = `{"services":{
	"web":{"image":"node:22"},
	"db":{"image":"postgres:16","environment":{"POSTGRES_USER":"app"}},
	"cache":{"image":"redis:7"},
	"geo":{"image":"postgis/postgis:16-3.4"}
}}`

var keyBackspace = tea.KeyMsg{Type: tea.KeyBackspace}

// wizardModel is a model with one git project that has a compose file and an ignored .env, plus a
// fake docker answering with composeJSON.
func wizardModel(t *testing.T, config string) model {
	t.Helper()
	isolateTUI(t)
	dir := newGitRepo(t)
	bin := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(bin, []byte(fakeComposeDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	old := docker.Binary
	docker.Binary = bin
	t.Cleanup(func() { docker.Binary = old })
	t.Setenv("FAKE_COMPOSE", composeJSON)
	for name, content := range map[string]string{
		"compose.yaml": "services: {}\n", ".gitignore": ".env*\n", ".lazydev/config.yaml": config,
		".env":      "PORT=3000\nSECRET=x\nDATABASE_URL=postgres://app@localhost/app\nPG_DB_NAME=app\n",
		".env.test": "DB=app_test\n", ".envrc.bak": "",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// .env.tracked is no candidate, git does not ignore it.
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env*\n!.env.tracked\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".env.tracked"), []byte("A=1\n"), 0o644)
	p := &projectEntry{path: dir}
	m := model{width: 160, height: 40, focus: colLeft, projects: []*projectEntry{p}, gitTabs: map[int]string{}}
	m.openWizard()
	if m.wizard == nil {
		t.Fatalf("wizard did not open: %s", m.err)
	}
	return m
}

func plainView(m model) string { return ansi.Strip(m.View()) }

func TestWizardDatabasesNavigation(t *testing.T) {
	m := wizardModel(t, "")
	// Without marked knowledge enter goes from Knowledge straight to Databases.
	m = press(t, m, keyEnter, keyEnter)
	if m.wizard.page != pageDatabases {
		t.Fatalf("page %d", m.wizard.page)
	}
	view := plainView(m)
	for _, want := range []string{"4/6 Databases", "Each worktree gets an empty database. Migrate in the setup hook.",
		"db   postgres:16", "geo  postgis/postgis:16-3.4", "↑↓ select · a add · d remove · enter next · backspace back · esc cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "redis") || strings.Contains(view, "node:22") {
		t.Errorf("services without Postgres show:\n%s", view)
	}
	if m = press(t, m, keyBackspace); m.wizard.page != pageKnowledge {
		t.Errorf("backspace went to page %d", m.wizard.page)
	}
	m = press(t, m, keyEnter, keyEnter)
	if m.wizard.page != pagePrompts || !strings.Contains(plainView(m), "5/6 Prompts") {
		t.Errorf("enter went to page %d", m.wizard.page)
	}
	if m = press(t, m, keyBackspace); m.wizard.page != pageDatabases {
		t.Errorf("backspace from prompts went to page %d", m.wizard.page)
	}

	// With marked knowledge the steps come first, backspace goes back to them.
	m.wizard.page = pageKnowledge
	m = press(t, m, keyRunes(" "), keyEnter)
	if m.wizard.page != pageSteps {
		t.Fatalf("page %d", m.wizard.page)
	}
	if m = press(t, m, keyEnter); m.wizard.page != pageDatabases {
		t.Errorf("enter from steps went to page %d", m.wizard.page)
	}
	if m = press(t, m, keyBackspace); m.wizard.page != pageSteps {
		t.Errorf("backspace went to page %d", m.wizard.page)
	}
	if m = press(t, m, keyEnter, keyEsc); m.wizard != nil {
		t.Error("esc did not close the wizard")
	}
}

func TestWizardStepsWithVerify(t *testing.T) {
	m := wizardModel(t, "")
	m = press(t, m, keyEnter, keyRunes(" "), keyEnter)
	if m.wizard.page != pageSteps {
		t.Fatalf("page %d", m.wizard.page)
	}
	view := plainView(m)
	for _, want := range []string{"1 create · 2 implement · 3 verify · 4 review · 5 merge · 6 unblock", "1-6 toggle step"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	f := m.wizard.markedFiles()[0]
	m = press(t, m, keyRunes("3"), keyRunes("6"))
	k := m.wizard.knowledge()
	for _, step := range []string{"create", "implement", "verify", "unblock"} {
		if !slices.Contains(k[step], f) {
			t.Errorf("knowledge %s lacks %s: %v", step, f, k)
		}
	}
	if slices.Contains(k["review"], f) || slices.Contains(k["merge"], f) {
		t.Errorf("knowledge: %v", k)
	}
	// All six steps go under all.
	m = press(t, m, keyRunes("4"), keyRunes("5"))
	if k := m.wizard.knowledge(); !slices.Equal(k["all"], []string{f}) || len(k) != 1 {
		t.Errorf("knowledge with all steps: %v", k)
	}
}

func TestWizardVerifyPrompt(t *testing.T) {
	m := wizardModel(t, "")
	m = press(t, m, keyEnter, keyEnter, keyEnter)
	if m.wizard.page != pagePrompts {
		t.Fatalf("page %d", m.wizard.page)
	}
	// Init leaves verify.md out, the step is off.
	if view := plainView(m); !strings.Contains(view, ".lazydev/verify.md") || !strings.Contains(view, "missing, step off") {
		t.Errorf("view:\n%s", view)
	}
	i := slices.Index(repo.Steps, repo.StepVerify)
	for range i {
		m = press(t, m, keyRunes("j"))
	}
	// r writes the template right away, the prompt is missing.
	m = press(t, m, keyRunes("r"))
	data, err := os.ReadFile(repo.PromptPath(m.wizard.path, repo.StepVerify))
	if err != nil || !strings.Contains(string(data), "$EVIDENCE") {
		t.Fatalf("verify.md = %q, %v", data, err)
	}
	if view := plainView(m); strings.Contains(view, "missing, step off") {
		t.Errorf("view after r:\n%s", view)
	}
}

func TestWizardLanguage(t *testing.T) {
	m := wizardModel(t, "merge: pr\n")
	if view := plainView(m); !strings.Contains(view, "Language: en") {
		t.Errorf("page 1 lacks the language:\n%s", view)
	}
	if keys := m.wizard.keys(); !strings.Contains(keys, "l language") {
		t.Errorf("footer: %q", keys)
	}
	m = press(t, m, keyRunes("l"))
	if view := plainView(m); !strings.Contains(view, "Language: de") {
		t.Errorf("l did not switch:\n%s", view)
	}
	// o and r on the page Prompts take the template of the chosen language.
	m = press(t, m, keyEnter, keyEnter, keyEnter)
	if m.wizard.page != pagePrompts {
		t.Fatalf("page %d", m.wizard.page)
	}
	m = press(t, m, keyRunes("r"), keyRunes("y"))
	if data, _ := os.ReadFile(repo.PromptPath(m.wizard.path, "create")); !strings.HasPrefix(string(data), "# PRD erstellen") {
		t.Errorf("create.md after r: %q", data)
	}
	m = press(t, m, keyEnter)
	if view := plainView(m); !strings.Contains(view, "Language: de") || !strings.Contains(view, "language: de") {
		t.Errorf("summary:\n%s", view)
	}
	m = press(t, m, keyEnter)
	if got, _ := os.ReadFile(repo.ConfigPath(m.projects[0].path)); string(got) != "language: de\nmerge: pr\n" {
		t.Errorf("config.yaml = %q", got)
	}

	// Back to en removes the field.
	m.openWizard()
	if m.wizard.lang != "de" {
		t.Fatalf("wizard lang %q", m.wizard.lang)
	}
	m = press(t, m, keyRunes("l"), keyEnter, keyEnter, keyEnter, keyEnter, keyEnter)
	if m.wizard != nil {
		t.Fatalf("wizard still open: %s", m.err)
	}
	if got, _ := os.ReadFile(repo.ConfigPath(m.projects[0].path)); string(got) != "merge: pr\n" {
		t.Errorf("config.yaml = %q", got)
	}
}

func TestWizardDatabasesAddRemove(t *testing.T) {
	m := wizardModel(t, "")
	m = press(t, m, keyEnter, keyEnter)
	// Rows are sorted: db, geo. a on db opens the env files.
	m = press(t, m, keyRunes("a"))
	p := m.wizard.dbPick
	if p == nil || !slices.Equal(p.options, []string{".env", ".env.test", ".envrc.bak"}) {
		t.Fatalf("env files: %+v", p)
	}
	if view := plainView(m); !strings.Contains(view, "Env file for db") {
		t.Errorf("view:\n%s", view)
	}
	// esc goes back without a change, the wizard stays.
	if m = press(t, m, keyEsc); m.wizard == nil || m.wizard.dbPick != nil || len(m.wizard.databases) != 0 {
		t.Fatalf("esc in the choice: %+v", m.wizard)
	}
	m = press(t, m, keyRunes("a"), keyEnter)
	if p := m.wizard.dbPick; p == nil || !slices.Equal(p.options, []string{"DATABASE_URL", "PG_DB_NAME", "PORT", "SECRET"}) {
		t.Fatalf("keys: %+v", p)
	}
	m = press(t, m, keyEnter)
	want := repo.Database{Service: "db", Env: repo.DatabaseEnv{File: ".env", Key: "DATABASE_URL"}}
	if m.wizard.dbPick != nil || !slices.Equal(m.wizard.databases, []repo.Database{want}) {
		t.Fatalf("databases: %+v", m.wizard.databases)
	}
	if view := plainView(m); !strings.Contains(view, "→ .env DATABASE_URL") {
		t.Errorf("view:\n%s", view)
	}
	// The same entry again is not added twice.
	m = press(t, m, keyRunes("a"), keyEnter, keyEnter)
	if len(m.wizard.databases) != 1 {
		t.Errorf("duplicate added: %+v", m.wizard.databases)
	}
	// d on a service row does nothing, on the entry row it removes the entry.
	m = press(t, m, keyRunes("d"))
	if len(m.wizard.databases) != 1 {
		t.Error("d on a service row removed something")
	}
	m = press(t, m, keyRunes("j"), keyRunes("d"))
	if len(m.wizard.databases) != 0 {
		t.Errorf("d did not remove: %+v", m.wizard.databases)
	}
}

func TestWizardDatabasesSave(t *testing.T) {
	config := "# lazydev\nhooks:\n  # migrate\n  setup: ./setup.sh\n"
	m := wizardModel(t, config)
	m = press(t, m, keyEnter, keyEnter)
	// geo gets .env.test DB, db gets .env DATABASE_URL: the file follows the order of the page.
	m = press(t, m, keyRunes("j"), keyRunes("a"), keyRunes("j"), keyEnter, keyEnter)
	m = press(t, m, keyRunes("k"), keyRunes("k"), keyRunes("a"), keyEnter, keyEnter)
	m = press(t, m, keyEnter, keyEnter)
	if m.wizard.page != pageSummary || !m.wizard.changed {
		t.Fatalf("summary: page %d, %+v", m.wizard.page, m.wizard.err)
	}
	if view := plainView(m); !strings.Contains(view, "databases:") {
		t.Errorf("summary lacks databases:\n%s", view)
	}
	m = press(t, m, keyEnter)
	if m.wizard != nil {
		t.Fatalf("wizard still open: %s", m.err)
	}
	got, _ := os.ReadFile(repo.ConfigPath(m.projects[0].path))
	want := config + "databases:\n  - service: db\n    env:\n      file: .env\n      key: DATABASE_URL\n" +
		"  - service: geo\n    env:\n      file: .env.test\n      key: DB\n"
	if string(got) != want {
		t.Errorf("config.yaml:\n%s\nwant\n%s", got, want)
	}

	// Opening again without a change writes nothing.
	m.openWizard()
	m = press(t, m, keyEnter, keyEnter, keyEnter, keyEnter)
	if m.wizard.changed {
		t.Errorf("unchanged databases count as a change:\n%s", m.wizard.preview)
	}
}

func TestWizardDatabasesKeepsUnknownService(t *testing.T) {
	config := "databases:\n  - service: old\n    env: {file: .env, key: DB}\n" +
		"  - service: db-test\n    compose: dev.yml\n    env: {file: .env.test, key: DB}\n"
	m := wizardModel(t, config)
	m = press(t, m, keyEnter, keyEnter)
	view := plainView(m)
	for _, want := range []string{"old (not in compose file)", "→ .env DB"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	// db-test is no service of dev.yml in the fake, db and geo of dev.yml show with their compose.
	services := map[string]bool{}
	for _, s := range m.wizard.dbServices {
		services[s.name+"|"+s.compose] = s.missing
	}
	if missing, ok := services["db|dev.yml"]; !ok || missing {
		t.Errorf("services: %v", services)
	}
	if missing := services["db-test|dev.yml"]; !missing {
		t.Errorf("services: %v", services)
	}
	m = press(t, m, keyEnter, keyEnter)
	if m.wizard.changed {
		t.Errorf("entries changed:\n%s", m.wizard.preview)
	}
}

func TestWizardDatabasesWithoutCompose(t *testing.T) {
	m := wizardModel(t, "")
	os.Remove(filepath.Join(m.projects[0].path, "compose.yaml"))
	m.wizard = nil
	m.openWizard()
	m = press(t, m, keyEnter, keyEnter)
	// The message wraps at the width of the wizard.
	if view := plainView(m); !strings.Contains(view, "No compose.yaml or docker-compose.yml in the main folder. Set") ||
		!strings.Contains(view, "compose: in config.yaml for other files.") {
		t.Errorf("view:\n%s", view)
	}
	if m = press(t, m, keyEnter); m.wizard.page != pagePrompts {
		t.Errorf("enter went to page %d", m.wizard.page)
	}
}

func TestWizardDatabasesWithoutPostgres(t *testing.T) {
	m := wizardModel(t, "")
	t.Setenv("FAKE_COMPOSE", `{"services":{"web":{"image":"node"}}}`)
	m.wizard = nil
	m.openWizard()
	m = press(t, m, keyEnter, keyEnter)
	if view := plainView(m); !strings.Contains(view, "No Postgres service in the compose file.") {
		t.Errorf("view:\n%s", view)
	}
}

func TestWizardDatabasesBrokenCompose(t *testing.T) {
	m := wizardModel(t, "databases:\n  - service: db\n    compose: broken.yml\n    env: {file: .env, key: DB}\n")
	m = press(t, m, keyEnter, keyEnter)
	view := plainView(m)
	if !strings.Contains(view, "docker compose: yaml: line 3: mapping values are not allowed") || strings.Contains(view, "second") {
		t.Errorf("view:\n%s", view)
	}
}

func TestWizardDatabasesWithoutEnvFile(t *testing.T) {
	m := wizardModel(t, "")
	dir := m.projects[0].path
	for _, f := range []string{".env", ".env.test", ".envrc.bak"} {
		os.Remove(filepath.Join(dir, f))
	}
	m = press(t, m, keyEnter, keyEnter, keyRunes("a"))
	if m.err != "No ignored .env file in the main folder." || m.wizard.dbPick != nil {
		t.Errorf("err %q, pick %+v", m.err, m.wizard.dbPick)
	}
}
