package runner

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lazydev/internal/fsutil"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// databasesConfig creates two databases in the service db: one from a URL, one from a plain name.
// The setup hook logs the database name it finds in .env of the worktree.
const databasesConfig = `databases:
  - service: db
    env:
      file: .env
      key: DATABASE_URL
  - service: db
    env:
      file: .env.test
      key: NUXT_DATABASE_NAME
hooks:
  setup: dbsetup.sh
  check: check.sh
  teardown: teardown.sh
`

const dbSetupScript = `#!/bin/sh
echo "setup $(sed -n 's/.*5432\/\([a-z_]*\).*/\1/p' .env)" >> "$LAZYDEV_PROJECT/hooks.log"
`

const (
	mainEnv     = "# app\nDATABASE_URL=\"postgres://app:secret@localhost:5432/app?sslmode=disable\"\n"
	mainEnvTest = "NUXT_DATABASE_NAME=app_test\n"
)

// newDatabaseProject is a trusted project with databasesConfig, the env files in the main folder,
// a fake docker without running container and the main database app.
func newDatabaseProject(t *testing.T) (Target, string) {
	t.Helper()
	d := fakeDocker(t)
	target := newProject(t, databasesConfig, false)
	write(t, target.Dir, "dbsetup.sh", dbSetupScript)
	if err := os.Chmod(filepath.Join(target.Dir, "dbsetup.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, target.Dir, "add", "dbsetup.sh")
	runGit(t, target.Dir, "commit", "-q", "-m", "dbsetup")
	write(t, target.Dir, ".env", mainEnv)
	write(t, target.Dir, ".env.test", mainEnvTest)
	write(t, filepath.Join(d, "dbs", "c-db"), "app", "")
	trustProject(t, target)
	return target, d
}

func dbExists(d, name string) bool {
	return fsutil.Exists(filepath.Join(d, "dbs", "c-db", name))
}

func dockerCalls(t *testing.T, d string) string {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(d, "calls"))
	return string(data)
}

func hooksLog(t *testing.T, target Target) []string {
	t.Helper()
	return strings.Split(strings.TrimSpace(read(t, filepath.Join(target.Dir, "hooks.log"))), "\n")
}

func TestSetupCreatesDatabases(t *testing.T) {
	target, d := newDatabaseProject(t)
	failAtReview(t, target)
	wt := WorktreePath("app", prdName)

	if got := read(t, filepath.Join(wt, ".env")); got != "# app\nDATABASE_URL=\"postgres://app:secret@localhost:5432/app_new_token?sslmode=disable\"\n" {
		t.Errorf(".env in the worktree:\n%s", got)
	}
	if got := read(t, filepath.Join(wt, ".env.test")); got != "NUXT_DATABASE_NAME=\"app_test_new_token\"\n" {
		t.Errorf(".env.test in the worktree:\n%s", got)
	}
	if got := read(t, filepath.Join(target.Dir, ".env")); got != mainEnv {
		t.Errorf(".env in the main folder changed:\n%s", got)
	}
	if !dbExists(d, "app_new_token") || !dbExists(d, "app_test_new_token") {
		t.Error("databases not created")
	}
	// The stopped service started once, in the main folder.
	if calls := dockerCalls(t, d); strings.Count(calls, "compose up -d db\n") != 1 {
		t.Errorf("calls:\n%s", calls)
	}
	// The setup hook ran after the databases and saw the new name.
	if got := hooksLog(t, target); got[0] != "setup app_new_token" {
		t.Errorf("hooks.log: %q", got)
	}
	r := loadRun(t)
	if want := []state.Database{{Service: "db", Name: "app_new_token"}, {Service: "db", Name: "app_test_new_token"}}; !slices.Equal(r.Databases, want) {
		t.Errorf("run state: %+v", r.Databases)
	}
	log := read(t, state.LogPath("app", prdName, state.StepSetup))
	for _, line := range []string{".env: DATABASE_URL → app_new_token", "Starting service db: docker compose up -d db", "Database app_new_token in c-db created"} {
		if !strings.Contains(log, line) {
			t.Errorf("setup log misses %q:\n%s", line, log)
		}
	}
}

func TestSetupRepeatKeepsDatabases(t *testing.T) {
	target, d := newDatabaseProject(t)
	// The setup hook fails once after the databases exist.
	write(t, target.Dir, "dbsetup.sh", "#!/bin/sh\necho setup >> \"$LAZYDEV_PROJECT/hooks.log\"\n[ -f \"$LAZYDEV_PROJECT/.checked\" ] || { touch \"$LAZYDEV_PROJECT/.checked\"; exit 1; }\n")
	runGit(t, target.Dir, "commit", "-q", "-am", "failing setup")
	if err := Run(context.Background(), target, prdName); err == nil || !strings.Contains(err.Error(), "setup hook ended with exit code 1") {
		t.Fatalf("Run: %v", err)
	}
	if !dbExists(d, "app_new_token") {
		t.Fatal("database missing after the failed setup hook")
	}
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if calls := dockerCalls(t, d); strings.Count(calls, "createdb -U app app_new_token") != 1 {
		t.Errorf("calls:\n%s", calls)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepSetup)); !strings.Contains(log, "Database app_new_token in c-db exists already") {
		t.Errorf("setup log:\n%s", log)
	}
}

func TestSetupEnvFileNotIgnored(t *testing.T) {
	target, d := newDatabaseProject(t)
	write(t, target.Dir, ".gitignore", "hooks.log\n.checked\n")
	runGit(t, target.Dir, "commit", "-q", "-am", "track env")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != ".env is not ignored by git, a worktree copy would dirty the worktree" {
		t.Fatalf("Run: %v", err)
	}
	if fsutil.Exists(filepath.Join(WorktreePath("app", prdName), ".env")) || dbExists(d, "app_new_token") {
		t.Error(".env written or database created")
	}
	if fsutil.Exists(filepath.Join(target.Dir, "hooks.log")) {
		t.Error("setup hook ran")
	}
}

func TestSetupDatabasesNeedTrust(t *testing.T) {
	target, d := newDatabaseProject(t)
	// A changed list during the run is not trusted.
	write(t, target.Dir, ".lazydev/config.yaml", strings.Replace(databasesConfig, "NUXT_DATABASE_NAME", "DB_NAME", 1))
	r := &state.Run{Worktree: t.TempDir(), Slug: "new_token"}
	if err := prepareDatabases(target, prdName, r, &logger{}); err == nil || err.Error() != "databases not created, databases in .lazydev/config.yaml are not trusted" {
		t.Errorf("prepareDatabases: %v", err)
	}
	if calls := dockerCalls(t, d); calls != "" {
		t.Errorf("calls:\n%s", calls)
	}
}

func TestMergeWithoutMovedBaseKeepsDatabases(t *testing.T) {
	target, d := newDatabaseProject(t)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	log := read(t, state.LogPath("app", prdName, state.StepMerge))
	if strings.Contains(log, "recreating databases") {
		t.Errorf("merge log:\n%s", log)
	}
	if got := hooksLog(t, target); slices.ContainsFunc(got[1:], func(s string) bool { return strings.HasPrefix(s, "setup") }) {
		t.Errorf("setup hook ran again: %q", got)
	}
	// The merge dropped the databases of the worktree, the main database stays.
	if dbExists(d, "app_new_token") || dbExists(d, "app_test_new_token") || !dbExists(d, "app") {
		t.Error("wrong databases after the merge")
	}
	if !strings.Contains(log, "Database app_new_token in c-db dropped") {
		t.Errorf("merge log:\n%s", log)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged || r.Note != "" {
		t.Errorf("state: %+v", r)
	}
}

func TestMergeWithMovedBaseRecreatesDatabases(t *testing.T) {
	target, d := newDatabaseProject(t)
	failAtReview(t, target)
	commitOnMain(t, target, "other.txt")
	before := strings.Count(dockerCalls(t, d), "createdb")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	log := read(t, state.LogPath("app", prdName, state.StepMerge))
	recreate := strings.Index(log, "main moved, recreating databases")
	if recreate < 0 || !strings.Contains(log, "Database app_new_token in c-db dropped") {
		t.Fatalf("merge log:\n%s", log)
	}
	if strings.Index(log, "check.sh") < recreate {
		t.Errorf("check ran before the databases:\n%s", log)
	}
	if got := strings.Count(dockerCalls(t, d), "createdb") - before; got != 2 {
		t.Errorf("%d databases created again", got)
	}
	// The setup hook ran again before the check.
	if got := hooksLog(t, target); len(got) < 3 || got[1] != "setup app_new_token" || !strings.HasPrefix(got[2], "check ") {
		t.Errorf("hooks.log: %q", got)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged || r.Note != "" {
		t.Errorf("state: %+v", r)
	}
}

func TestDiscardDropsDatabases(t *testing.T) {
	target, d := newDatabaseProject(t)
	failAtReview(t, target)
	if err := Discard(target, prdName); err != nil {
		t.Fatal(err)
	}
	if dbExists(d, "app_new_token") || dbExists(d, "app_test_new_token") || !dbExists(d, "app") {
		t.Error("wrong databases after X")
	}
}

func TestDiscardWithoutStateDropsDatabasesOfConfig(t *testing.T) {
	target, d := newDatabaseProject(t)
	failAtReview(t, target)
	// Without run state and without worktree the names come from the main folder.
	if err := removeWorktree(target.Dir, WorktreePath("app", prdName), "feat/"+prdName, true); err != nil {
		t.Fatal(err)
	}
	if err := state.Remove("app", prdName); err != nil {
		t.Fatal(err)
	}
	if err := Discard(target, prdName); err != nil {
		t.Fatal(err)
	}
	if dbExists(d, "app_new_token") || dbExists(d, "app_test_new_token") || !dbExists(d, "app") {
		t.Error("wrong databases after X")
	}
}

func TestDiscardWithoutTrustKeepsDatabases(t *testing.T) {
	target, d := newDatabaseProject(t)
	failAtReview(t, target)
	write(t, target.Dir, ".lazydev/config.yaml", databasesConfig+"allowedTools:\n  - Bash(x)\n")
	err := Discard(target, prdName)
	if err == nil || !strings.Contains(err.Error(), "databases not dropped, databases in .lazydev/config.yaml are not trusted") {
		t.Fatalf("Discard: %v", err)
	}
	if !dbExists(d, "app_new_token") || !dbExists(d, "app_test_new_token") {
		t.Error("databases dropped without trust")
	}
	if strings.Contains(dockerCalls(t, d), "dropdb") {
		t.Error("dropdb called")
	}
}

func TestDiscardProtectsMainDatabase(t *testing.T) {
	target, d := newDatabaseProject(t)
	failAtReview(t, target)
	r := loadRun(t)
	r.Databases = []state.Database{{Service: "db", Name: "app"}}
	if err := state.Save("app", prdName, r); err != nil {
		t.Fatal(err)
	}
	err := Discard(target, prdName)
	if err == nil || !strings.Contains(err.Error(), "database app does not belong to the worktree, not dropping it") {
		t.Fatalf("Discard: %v", err)
	}
	if !dbExists(d, "app") {
		t.Error("main database dropped")
	}
}
