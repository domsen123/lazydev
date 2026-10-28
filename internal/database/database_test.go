package database

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lazydev/internal/docker"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// fakeDocker imitates docker with files in $FAKE_DOCKER: up/<service> holds the working dir of a
// running container c-<service>, dbs/<container>/<name> a database. calls logs every call.
// compose config answers with compose.json if it exists.
const fakeDocker = `#!/bin/sh
d="$FAKE_DOCKER"
printf '%s\n' "$*" >> "$d/calls"
case "$1" in
ps)
	for f in "$d"/up/*; do
		[ -f "$f" ] || continue
		s=$(basename "$f")
		printf '{"Names":"c-%s","State":"running","Labels":"com.docker.compose.service=%s,com.docker.compose.project.working_dir=%s"}\n' "$s" "$s" "$(cat "$f")"
	done ;;
compose)
	shift
	wd="$PWD"
	if [ "$1" = -f ]; then wd=$(cd "$(dirname "$2")" && pwd); shift 2; fi
	case "$1" in
	config) if [ -f "$d/compose.json" ]; then cat "$d/compose.json"; else echo '{"services":{"db":{"image":"postgres:16","environment":{"POSTGRES_USER":"app"}}}}'; fi ;;
	up) eval s=\${$#}; mkdir -p "$d/up"; printf '%s' "$wd" > "$d/up/$s" ;;
	esac ;;
exec)
	c="$2"; shift 2
	eval last=\${$#}
	mkdir -p "$d/dbs/$c"
	case "$1" in
	pg_isready) [ -f "$d/notready" ] && exit 2; exit 0 ;;
	psql) n=$(printf '%s' "$last" | sed "s/.*datname = '\(.*\)'\$/\1/"); [ -f "$d/dbs/$c/$n" ] && echo 1; exit 0 ;;
	createdb) touch "$d/dbs/$c/$last" ;;
	dropdb) rm -f "$d/dbs/$c/$last" ;;
	esac ;;
esac
`

// fake replaces docker.Binary with fakeDocker and returns its folder.
func fake(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "docker")
	if err := os.WriteFile(bin, []byte(fakeDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_DOCKER", dir)
	old, oldTimeout, oldPoll := docker.Binary, ReadyTimeout, pollInterval
	docker.Binary, ReadyTimeout, pollInterval = bin, 50*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { docker.Binary, ReadyTimeout, pollInterval = old, oldTimeout, oldPoll })
	return dir
}

func dbEntry(service, file, key string) repo.Database {
	return repo.Database{Service: service, Env: repo.DatabaseEnv{File: file, Key: key}}
}

// project creates a main folder with .env and .env.test and a worktree repo that ignores .env*.
func project(t *testing.T) (main, wt string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	main, wt = t.TempDir(), t.TempDir()
	write(t, main, ".env", "# app\nDATABASE_URL=\"postgres://app:secret@localhost:5432/app?sslmode=disable\"\nPORT=3000\n")
	write(t, main, ".env.test", "NUXT_DATABASE_NAME=app_test\n")
	if out, err := exec.Command("git", "-C", wt, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	write(t, wt, ".gitignore", ".env*\n")
	return main, wt
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(dir, container, name string) bool {
	_, err := os.Stat(filepath.Join(dir, "dbs", container, name))
	return err == nil
}

// logs collects the lines of a Logger.
type logs []string

func (l *logs) log(format string, args ...any) { *l = append(*l, fmt.Sprintf(format, args...)) }

func TestPrepareStartsServiceAndCreates(t *testing.T) {
	d := fake(t)
	main, wt := project(t)
	dbs := []repo.Database{dbEntry("db", ".env", "DATABASE_URL"), dbEntry("db", ".env.test", "NUXT_DATABASE_NAME")}
	var l logs
	got, err := Prepare(main, wt, "login_form", dbs, l.log)
	if err != nil {
		t.Fatal(err)
	}
	want := []state.Database{{Service: "db", Name: "app_login_form"}, {Service: "db", Name: "app_test_login_form"}}
	if !slices.Equal(got, want) {
		t.Errorf("Prepare = %+v", got)
	}
	data, _ := os.ReadFile(filepath.Join(wt, ".env"))
	if string(data) != "# app\nDATABASE_URL=\"postgres://app:secret@localhost:5432/app_login_form?sslmode=disable\"\nPORT=3000\n" {
		t.Errorf(".env:\n%s", data)
	}
	if info, _ := os.Stat(filepath.Join(wt, ".env")); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}
	if data, _ := os.ReadFile(filepath.Join(wt, ".env.test")); string(data) != "NUXT_DATABASE_NAME=\"app_test_login_form\"\n" {
		t.Errorf(".env.test:\n%s", data)
	}
	if !exists(d, "c-db", "app_login_form") || !exists(d, "c-db", "app_test_login_form") {
		t.Error("databases not created")
	}
	for _, line := range []string{".env: DATABASE_URL → app_login_form", "Starting service db: docker compose up -d db",
		"Database app_login_form in c-db created", "Database app_test_login_form in c-db created"} {
		if !slices.Contains(l, line) {
			t.Errorf("log misses %q:\n%s", line, strings.Join(l, "\n"))
		}
	}
	calls, _ := os.ReadFile(filepath.Join(d, "calls"))
	if !strings.Contains(string(calls), "exec c-db createdb -U app app_login_form") || strings.Count(string(calls), "up -d") != 1 {
		t.Errorf("calls:\n%s", calls)
	}

	// Again: the databases stay, nothing starts.
	l = nil
	if _, err := Prepare(main, wt, "login_form", dbs, l.log); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(l, "Database app_login_form in c-db exists already") || slices.ContainsFunc(l, func(s string) bool { return strings.HasPrefix(s, "Starting") }) {
		t.Errorf("second Prepare:\n%s", strings.Join(l, "\n"))
	}
}

func TestPrepareSameNameOnce(t *testing.T) {
	d := fake(t)
	main, wt := project(t)
	write(t, main, ".env", "DB_NAME=app\nDATABASE_URL=postgres://h/app\n")
	var l logs
	got, err := Prepare(main, wt, "x", []repo.Database{dbEntry("db", ".env", "DB_NAME"), dbEntry("db", ".env", "DATABASE_URL")}, l.log)
	if err != nil || len(got) != 1 {
		t.Fatalf("Prepare = %+v, %v", got, err)
	}
	calls, _ := os.ReadFile(filepath.Join(d, "calls"))
	if strings.Count(string(calls), "createdb") != 1 {
		t.Errorf("calls:\n%s", calls)
	}
	if data, _ := os.ReadFile(filepath.Join(wt, ".env")); string(data) != "DB_NAME=\"app_x\"\nDATABASE_URL=\"postgres://h/app_x\"\n" {
		t.Errorf(".env:\n%s", data)
	}
}

func TestPrepareErrors(t *testing.T) {
	fake(t)
	main, wt := project(t)
	for _, tc := range []struct {
		db   repo.Database
		want string
	}{
		{dbEntry("db", ".env.missing", "X"), ".env.missing missing in the main folder"},
		{dbEntry("db", ".env", "MISSING"), ".env: MISSING is not set"},
	} {
		if _, err := Prepare(main, wt, "x", []repo.Database{tc.db}, func(string, ...any) {}); err == nil || err.Error() != tc.want {
			t.Errorf("error = %v, want %s", err, tc.want)
		}
	}
	write(t, main, "env.local", "DB=app\n")
	if _, err := Prepare(main, wt, "x", []repo.Database{dbEntry("db", "env.local", "DB")}, func(string, ...any) {}); err == nil ||
		err.Error() != "env.local is not ignored by git, a worktree copy would dirty the worktree" {
		t.Errorf("not ignored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "env.local")); err == nil {
		t.Error("env.local written")
	}
}

func TestPrepareNotReady(t *testing.T) {
	d := fake(t)
	main, wt := project(t)
	write(t, d, "notready", "")
	_, err := Prepare(main, wt, "x", []repo.Database{dbEntry("db", ".env", "DATABASE_URL")}, func(string, ...any) {})
	if err == nil || err.Error() != "postgres in service db not ready after 0s" {
		t.Errorf("error = %v", err)
	}
}

func TestPrepareComposeFile(t *testing.T) {
	d := fake(t)
	main, wt := project(t)
	if err := os.MkdirAll(filepath.Join(main, "dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, d, "compose.json", `{"services":{"db-test":{"image":"postgres"}}}`)
	db := repo.Database{Service: "db-test", Compose: "dev/compose.yml", Env: repo.DatabaseEnv{File: ".env.test", Key: "NUXT_DATABASE_NAME"}}
	var l logs
	if _, err := Prepare(main, wt, "x", []repo.Database{db}, l.log); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(l, "Starting service db-test: docker compose -f dev/compose.yml up -d db-test") {
		t.Errorf("log:\n%s", strings.Join(l, "\n"))
	}
	// Without POSTGRES_USER the user is postgres.
	calls, _ := os.ReadFile(filepath.Join(d, "calls"))
	if !strings.Contains(string(calls), "exec c-db-test createdb -U postgres app_test_x") {
		t.Errorf("calls:\n%s", calls)
	}
}

func TestRecreate(t *testing.T) {
	d := fake(t)
	main, wt := project(t)
	dbs := []repo.Database{dbEntry("db", ".env", "DATABASE_URL")}
	if _, err := Prepare(main, wt, "x", dbs, func(string, ...any) {}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(d, "dbs", "c-db"), "app_x", "data")
	var l logs
	if _, err := Recreate(main, wt, "x", dbs, l.log); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l[1:], []string{"Database app_x in c-db dropped", "Database app_x in c-db created"}) {
		t.Errorf("log:\n%s", strings.Join(l, "\n"))
	}
	if data, _ := os.ReadFile(filepath.Join(d, "dbs", "c-db", "app_x")); len(data) != 0 {
		t.Error("database not recreated empty")
	}
}

func TestDrop(t *testing.T) {
	d := fake(t)
	if err := os.MkdirAll(filepath.Join(d, "dbs", "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app", "app_x", "app_xy"} {
		write(t, filepath.Join(d, "dbs", "c"), name, "")
	}
	var l logs
	for _, tc := range []struct{ name, slug string }{{"app", "x"}, {"app_xy", "x"}, {"app_", ""}, {"app", ""}} {
		err := Drop(Target{Container: "c", User: "u", Name: tc.name}, tc.slug, l.log)
		if want := "database " + tc.name + " does not belong to the worktree, not dropping it"; err == nil || err.Error() != want {
			t.Errorf("Drop(%s, %q) = %v", tc.name, tc.slug, err)
		}
	}
	if err := Drop(Target{Container: "c", User: "u", Name: "app_x"}, "x", l.log); err != nil {
		t.Fatal(err)
	}
	if exists(d, "c", "app_x") || !exists(d, "c", "app") || !exists(d, "c", "app_xy") {
		t.Error("wrong databases dropped")
	}
	calls, _ := os.ReadFile(filepath.Join(d, "calls"))
	if string(calls) != "exec c dropdb --if-exists --force -U u app_x\n" {
		t.Errorf("calls:\n%s", calls)
	}
}

func TestDropAll(t *testing.T) {
	d := fake(t)
	main, wt := project(t)
	dbs := []repo.Database{dbEntry("db", ".env", "DATABASE_URL"), dbEntry("db", ".env.test", "NUXT_DATABASE_NAME")}
	created, err := Prepare(main, wt, "x", dbs, func(string, ...any) {})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(d, "dbs", "c-db"), "app", "")
	var l logs
	if err := DropAll(main, "x", created, dbs, l.log); err != nil {
		t.Fatal(err)
	}
	if exists(d, "c-db", "app_x") || exists(d, "c-db", "app_test_x") || !exists(d, "c-db", "app") {
		t.Error("wrong databases dropped")
	}

	// Without run state the names come from the main folder, a missing key is reported.
	if _, err := Prepare(main, wt, "x", dbs, func(string, ...any) {}); err != nil {
		t.Fatal(err)
	}
	broken := append(slices.Clone(dbs), dbEntry("db", ".env", "MISSING"), dbEntry("db", ".env.gone", "X"))
	err = DropAll(main, "x", nil, broken, l.log)
	if err == nil || err.Error() != ".env: database not determinable\n.env.gone: database not determinable" {
		t.Errorf("DropAll error = %v", err)
	}
	if exists(d, "c-db", "app_x") || exists(d, "c-db", "app_test_x") {
		t.Error("databases from config.yaml not dropped")
	}

	// A stopped service starts nothing and fails nothing.
	if err := os.Remove(filepath.Join(d, "up", "db")); err != nil {
		t.Fatal(err)
	}
	l = nil
	if err := DropAll(main, "x", created, dbs, l.log); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l, []string{"Service db is not running, database app_x stays", "Service db is not running, database app_test_x stays"}) {
		t.Errorf("log:\n%s", strings.Join(l, "\n"))
	}
	calls, _ := os.ReadFile(filepath.Join(d, "calls"))
	if strings.Count(string(calls), "up -d") != 1 {
		t.Errorf("calls:\n%s", calls)
	}
}
