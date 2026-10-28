package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/repo"
)

func projectInfo(dbs ...repo.Database) repo.Info {
	return repo.Info{Initialized: true, Prompts: make([]bool, len(repo.Steps)),
		Config: repo.Config{Hooks: repo.Hooks{Setup: "./setup.sh"}, Databases: dbs}}
}

func TestTrustSummaryDatabases(t *testing.T) {
	info := projectInfo(repo.Database{Service: "db"}, repo.Database{Service: "db-test"})
	if got := trustSummary(info.Config); got != "setup ./setup.sh, 2 databases" {
		t.Errorf("trustSummary = %q", got)
	}
}

func TestProjectDetailsDatabases(t *testing.T) {
	info := projectInfo(
		repo.Database{Service: "db", Env: repo.DatabaseEnv{File: ".env", Key: "DATABASE_URL"}},
		repo.Database{Service: "db-test", Compose: "dev.yml", Env: repo.DatabaseEnv{File: ".env.test", Key: "DB_NAME"}},
	)
	view := ansi.Strip(projectDetailsView(info, 200, 8, themes[0]))
	lines := strings.Split(view, "\n")
	hooks, dbs, knowledge := strings.Index(lines[0], "Hooks"), strings.Index(lines[0], "Databases"), strings.Index(lines[0], "Knowledge")
	if hooks < 0 || dbs < hooks || knowledge < dbs {
		t.Fatalf("columns out of order:\n%s", view)
	}
	for _, want := range []string{"db → .env DATABASE_URL", "db-test → .env.test DB_NAME"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}

	// Without entries the column shows a dash.
	view = ansi.Strip(projectDetailsView(projectInfo(), 200, 8, themes[0]))
	lines = strings.Split(view, "\n")
	col := len([]rune(lines[0][:max(0, strings.Index(lines[0], "Databases"))]))
	if col == 0 || !strings.HasPrefix(string([]rune(lines[1])[col:]), "–") {
		t.Errorf("empty column:\n%s", view)
	}

	// Too narrow, the column drops from the right like the others.
	view = ansi.Strip(projectDetailsView(info, 60, 8, themes[0]))
	if strings.Contains(view, "Databases") {
		t.Errorf("narrow view shows Databases:\n%s", view)
	}
}
