package docker

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestHostPorts(t *testing.T) {
	for ports, want := range map[string][]int{
		"":         nil,
		"5432/tcp": nil,
		"0.0.0.0:5435->5432/tcp, [::]:5435->5432/tcp":                     {5435},
		"0.0.0.0:3000->3000/tcp, 0.0.0.0:8080->80/tcp, [::]:8080->80/tcp": {3000, 8080},
		"127.0.0.1:6379->6379/tcp, 9000/tcp":                              {6379},
		"[::]:bad->1/tcp":                                                 nil,
	} {
		if got := (Container{Ports: ports}).HostPorts(); !slices.Equal(got, want) {
			t.Errorf("HostPorts(%q) = %v, want %v", ports, got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	c := Container{Labels: "com.docker.compose.service=db,com.docker.compose.project.working_dir=/p/shop,x=a=b"}
	if got := c.label(labelService); got != "db" {
		t.Errorf("service: %q", got)
	}
	if got := c.label(labelWorkingDir); got != "/p/shop" {
		t.Errorf("working_dir: %q", got)
	}
	if got := c.label("x"); got != "a=b" {
		t.Errorf("value with =: %q", got)
	}
	if got := c.label("missing"); got != "" {
		t.Errorf("missing: %q", got)
	}
}

func TestProblem(t *testing.T) {
	for _, tc := range []struct {
		c    Container
		want bool
	}{
		{Container{State: "running", Status: "Up 2 hours"}, false},
		{Container{State: "running", Health: "unhealthy"}, true},
		{Container{State: "running", Health: "healthy"}, false},
		{Container{State: "restarting"}, true},
		{Container{State: "dead"}, true},
		{Container{State: "exited", Status: "Exited (0) 3 minutes ago"}, false},
		{Container{State: "exited", Status: "Exited (137) 3 minutes ago"}, true},
		{Container{State: "created", Status: "Created"}, false},
	} {
		if got := tc.c.problem(); got != tc.want {
			t.Errorf("problem(%+v) = %v, want %v", tc.c, got, tc.want)
		}
	}
}

func compose(dir, service string) string {
	return labelWorkingDir + "=" + dir + "," + labelService + "=" + service
}

func TestProjectServices(t *testing.T) {
	info := Info{
		Services: map[string][]string{"/p/shop": {"web", "db", "cache"}},
		Containers: []Container{
			{Names: "shop-db-1", State: "running", Ports: "0.0.0.0:5435->5432/tcp, [::]:5435->5432/tcp", Labels: compose("/p/shop", "db")},
			{Names: "shop-web-1", State: "exited", Status: "Exited (1) 1 minute ago", Labels: compose("/p/shop", "web")},
			{Names: "shop-worker-1", State: "running", Labels: compose("/p/shop", "worker")},
			{Names: "manual", State: "running", Labels: labelWorkingDir + "=/p/shop"},
			{Names: "blog-db-1", State: "running", Labels: compose("/p/blog", "db")},
			{Names: "plain", State: "running"},
		},
	}
	got := info.ProjectServices("/p/shop")
	type row struct {
		name  string
		state State
		ports []int
		c     string
	}
	want := []row{
		{"web", Failed, nil, "shop-web-1"},
		{"db", Running, []int{5435}, "shop-db-1"},
		{"cache", Stopped, nil, ""},
		{"worker", Running, nil, "shop-worker-1"},
		{"manual", Running, nil, "manual"},
	}
	if len(got) != len(want) {
		t.Fatalf("services: %+v", got)
	}
	for i, w := range want {
		s := got[i]
		name := ""
		if s.Container != nil {
			name = s.Container.Names
		}
		if s.Name != w.name || s.State != w.state || !slices.Equal(s.Ports, w.ports) || name != w.c {
			t.Errorf("service %d: %+v (container %q), want %+v", i, s, name, w)
		}
	}
	// The container points into Info, not to a copy of the loop.
	if got[1].Container != &info.Containers[0] {
		t.Error("container is a copy")
	}
}

func TestProjectServicesStoppedContainer(t *testing.T) {
	info := Info{Containers: []Container{
		{Names: "x", State: "exited", Status: "Exited (0) 1 hour ago", Labels: compose("/p", "job")},
	}}
	got := info.ProjectServices("/p")
	if len(got) != 1 || got[0].State != Stopped || got[0].Container == nil {
		t.Errorf("services: %+v", got)
	}
	if got := info.ProjectServices("/other"); len(got) != 0 {
		t.Errorf("other project: %+v", got)
	}
}

func TestHasCompose(t *testing.T) {
	dir := t.TempDir()
	if HasCompose(dir) {
		t.Error("compose file found in empty folder")
	}
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), nil, 0o644)
	if !HasCompose(dir) {
		t.Error("docker-compose.yml not found")
	}
}

func TestIsPostgres(t *testing.T) {
	for image, want := range map[string]bool{
		"postgres":                             true,
		"postgres:16-alpine":                   true,
		"docker.io/library/postgres:16":        true,
		"library/postgres":                     true,
		"postgis/postgis:16-3.4":               true,
		"pgvector/pgvector:pg16":               true,
		"timescale/timescaledb:latest-pg16":    true,
		"ghcr.io/acme/postgres@sha256:abc":     true,
		"localhost:5000/postgres:16":           true,
		"bitnami/postgresql:16":                false,
		"redis:7":                              false,
		"mysql":                                false,
		"postgres-exporter":                    false,
		"":                                     false,
		"registry.example.com:5000/x/pgvector": true,
	} {
		if got := IsPostgres(ComposeService{Image: image}); got != want {
			t.Errorf("IsPostgres(%q) = %v, want %v", image, got, want)
		}
	}
	if !IsPostgres(ComposeService{Image: "custom/db", Environment: map[string]string{"POSTGRES_PASSWORD": ""}}) {
		t.Error("POSTGRES_ variable not recognized")
	}
	if IsPostgres(ComposeService{Image: "custom/db", Environment: map[string]string{"PGDATA": "/d"}}) {
		t.Error("custom/db without POSTGRES_ recognized")
	}
}

func TestParseComposeConfig(t *testing.T) {
	data := `{"name":"app","services":{
		"db":{"image":"postgres:16","environment":{"POSTGRES_USER":"app","POSTGRES_PASSWORD":null}},
		"web":{"build":{"context":"."},"ports":[{"target":3000}]}
	}}`
	got, err := parseComposeConfig([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if db := got["db"]; db.Image != "postgres:16" || db.Environment["POSTGRES_USER"] != "app" || db.Environment["POSTGRES_PASSWORD"] != "" || len(db.Environment) != 2 {
		t.Errorf("db = %+v", db)
	}
	if web, ok := got["web"]; !ok || web.Image != "" || len(web.Environment) != 0 {
		t.Errorf("web = %+v", web)
	}
	if _, err := parseComposeConfig([]byte("services: x")); err == nil {
		t.Error("no error for broken JSON")
	}
}

// fakeDocker answers ps with one running container of /p and records its arguments.
const fakeDocker = `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_DOCKER_ARGS"
case "$1" in
ps) echo '{"Names":"p-db-1","State":"running","Labels":"com.docker.compose.service=db,com.docker.compose.project.working_dir=/p"}'
	echo '{"Names":"p-old-1","State":"exited","Labels":"com.docker.compose.service=old,com.docker.compose.project.working_dir=/p"}' ;;
exec) shift 2; [ "$1" = fail ] && { echo "boom" >&2; echo "second" >&2; exit 1; }; echo "out $*" ;;
compose) echo '{"services":{"db":{"image":"postgres"}}}' ;;
esac
`

func TestFakeBinary(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "docker")
	os.WriteFile(fake, []byte(fakeDocker), 0o755)
	t.Setenv("FAKE_DOCKER_ARGS", filepath.Join(dir, "args"))
	old := Binary
	Binary = fake
	t.Cleanup(func() { Binary = old })

	if name, ok, err := RunningContainer("/p", "db"); err != nil || !ok || name != "p-db-1" {
		t.Errorf("RunningContainer(db) = %q, %v, %v", name, ok, err)
	}
	for _, tc := range []struct{ dir, service string }{{"/p", "old"}, {"/q", "db"}, {"/p", "web"}} {
		if _, ok, err := RunningContainer(tc.dir, tc.service); err != nil || ok {
			t.Errorf("RunningContainer(%s, %s) = %v, %v", tc.dir, tc.service, ok, err)
		}
	}
	if out, err := Exec("p-db-1", "psql", "-c", "x"); err != nil || out != "out psql -c x\n" {
		t.Errorf("Exec = %q, %v", out, err)
	}
	if _, err := Exec("p-db-1", "fail"); err == nil || err.Error() != "docker exec: boom" {
		t.Errorf("Exec error = %v", err)
	}
	if err := Up(t.TempDir(), "dev.yml", "db"); err != nil {
		t.Error(err)
	}
	if s, err := ComposeConfig(t.TempDir(), ""); err != nil || s["db"].Image != "postgres" {
		t.Errorf("ComposeConfig = %+v, %v", s, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "args"))
	if !strings.Contains(string(data), "compose -f dev.yml up -d db\n") || !strings.Contains(string(data), "compose config --format json\n") {
		t.Errorf("args:\n%s", data)
	}
}

// TestWithoutDocker runs Load and Events with an empty PATH, so no docker daemon is involved.
func TestWithoutDocker(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "compose.yaml"), nil, 0o644)
	known := map[string][]string{"/known": {"a", "b"}}
	info := Load([]string{"/known", dir, t.TempDir()}, known)
	if info.Err == nil || info.Containers != nil {
		t.Errorf("Load without docker: %+v", info)
	}
	if !slices.Equal(info.Services["/known"], []string{"a", "b"}) {
		t.Errorf("known services: %v", info.Services)
	}
	// The compose file exists, but docker compose config fails without docker.
	if s, ok := info.Services[dir]; !ok || s != nil {
		t.Errorf("services of compose project: %v, %v", s, ok)
	}
	if len(info.Services) != 2 {
		t.Errorf("project without compose file listed: %v", info.Services)
	}
	if ch := Events(context.Background()); ch != nil {
		t.Error("Events without docker is not nil")
	}
}
