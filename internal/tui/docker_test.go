package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/docker"
)

func testDocker() dockerInfo {
	label := func(dir, svc string) string {
		return docker.LabelWorkingDir + "=" + dir + "," + docker.LabelService + "=" + svc
	}
	return dockerInfo{
		containers: []docker.Container{
			{Names: "app-postgres-1", State: "running", Ports: "0.0.0.0:5435->5432/tcp, [::]:5435->5432/tcp", Labels: label("/p/app", "postgres")},
			{Names: "app-mailpit-1", State: "running", Ports: "0.0.0.0:8025->8025/tcp, 0.0.0.0:1025->1025/tcp", Labels: label("/p/app", "mailpit")},
			{Names: "app-worker-1", State: "exited", Status: "Exited (2) 1 hour ago", Labels: label("/p/app", "worker")},
			{Names: "other-db-1", State: "running", Labels: label("/p/other", "db")},
		},
		services: map[string][]string{"/p/app": {"postgres", "mailpit", "redis", "worker"}},
		dev:      map[string][]devEntry{"/p/app": {{label: "main", port: 3000}, {label: "avatar", port: 3101}}},
	}
}

func TestDockerLine(t *testing.T) {
	d := testDocker()
	line, ok := d.dockerLine("/p/app", 200)
	if got := ansi.Strip(line); !ok || got != "docker ● postgres :5435   ● mailpit :8025 :1025   ○ redis   ✗ worker" {
		t.Errorf("Docker-Zeile: %q", got)
	}
	short, _ := d.dockerLine("/p/app", 40)
	if got := ansi.Strip(short); !strings.HasPrefix(got, "docker ◐ 2/4 · :5435 :8025") || ansi.StringWidth(short) > 40 {
		t.Errorf("gekürzt: %q", got)
	}
	if d.projectSymbol("/p/app") != "△" || d.projectSymbol("/p/other") != "▲" || d.projectSymbol("/p/none") != "" {
		t.Error("Projekt-Symbole falsch")
	}
	dev, ok := d.devLine("/p/app")
	if got := ansi.Strip(dev); !ok || got != "dev    ● main :3000   ● avatar :3101" {
		t.Errorf("Dev-Zeile: %q", got)
	}
	d.err = os.ErrNotExist
	if line, _ := d.dockerLine("/p/app", 200); !strings.Contains(ansi.Strip(line), "nicht erreichbar") {
		t.Errorf("ohne Docker: %q", line)
	}
}

func TestDockerBlock(t *testing.T) {
	d := testDocker()
	d.containers[0].Status, d.containers[0].Image = "Up 2 hours (healthy)", "postgres:16"
	lines := d.dockerBlock("/p/app", 120)
	if len(lines) != 5 {
		t.Fatalf("%d Zeilen: %q", len(lines), lines)
	}
	if got := ansi.Strip(lines[0]); !strings.Contains(got, "Status") || !strings.Contains(got, "Container") {
		t.Errorf("Kopfzeile: %q", got)
	}
	for _, want := range []string{"● postgres", "Up 2 hours (healthy)", ":5435", "postgres:16", "app-postgres-1"} {
		if !strings.Contains(ansi.Strip(lines[1]), want) {
			t.Errorf("postgres-Zeile ohne %q: %q", want, ansi.Strip(lines[1]))
		}
	}
	if got := ansi.Strip(lines[3]); !strings.HasPrefix(got, "○ redis") || !strings.Contains(got, "gestoppt") {
		t.Errorf("redis-Zeile: %q", got)
	}
	// Schmal fallen Image und Container weg.
	for _, l := range d.dockerBlock("/p/app", 60) {
		if w := ansi.StringWidth(l); w > 60 {
			t.Errorf("Zeile %d breit: %q", w, ansi.Strip(l))
		}
		if strings.Contains(ansi.Strip(l), "postgres:16") {
			t.Errorf("Image trotz schmaler Breite: %q", ansi.Strip(l))
		}
	}
	d.err = os.ErrNotExist
	if lines := d.dockerBlock("/p/app", 120); len(lines) != 1 || !strings.Contains(ansi.Strip(lines[0]), "nicht erreichbar") {
		t.Errorf("ohne Docker: %q", lines)
	}
}

// TestDockerBlockInPRDs prüft, dass der Docker-Block unten in der PRD-Box steht und bei wenig Höhe
// zur kurzen Zeile wird.
func TestDockerBlockInPRDs(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg, _ := config.Load(cfgPath)
	dir := newTestProject(t, root, "app")
	if _, err := cfg.AddProject(dir); err != nil {
		t.Fatal(err)
	}
	d := testDocker()
	for i := range d.containers {
		d.containers[i].Labels = strings.ReplaceAll(d.containers[i].Labels, "/p/app", dir)
	}
	d.services = map[string][]string{dir: d.services["/p/app"]}
	d.dev = map[string][]devEntry{dir: d.dev["/p/app"]}

	m := drive(t, New(cfgPath, cfg), tea.WindowSizeMsg{Width: 160, Height: 50})
	m = drive(t, m, m.Init()())
	m.docker = d
	l := m.layout()
	box := strings.Split(ansi.Strip(m.viewPRDs(l)), "\n")
	if len(box) != l.topHeight {
		t.Fatalf("%d Zeilen statt %d", len(box), l.topHeight)
	}
	if got := box[len(box)-2]; !strings.Contains(got, "dev    ● main :3000") {
		t.Errorf("vorletzte Zeile: %q", got)
	}
	if got := box[len(box)-3]; !strings.Contains(got, "✗ worker") {
		t.Errorf("letzte Docker-Zeile: %q", got)
	}
	if !strings.Contains(box[2], "auth/token") {
		t.Errorf("PRD fehlt oben: %q", box[2])
	}

	m = drive(t, m, tea.WindowSizeMsg{Width: 160, Height: 20})
	l = m.layout()
	box = strings.Split(ansi.Strip(m.viewPRDs(l)), "\n")
	if got := box[len(box)-3]; !strings.Contains(got, "docker ● postgres :5435") {
		t.Errorf("kurze Docker-Zeile fehlt: %q", box)
	}
	if m.prdRows(m.selectedProject(), l) < minPRDRows {
		t.Errorf("nur %d PRD-Zeilen", m.prdRows(m.selectedProject(), l))
	}
}

func TestEnvPort(t *testing.T) {
	dir := t.TempDir()
	if envPort(dir) != defaultDevPort {
		t.Error("Standardport fehlt")
	}
	os.WriteFile(filepath.Join(dir, ".env"), []byte("NUXT_X=1\nPORT=\"3004\"\n"), 0o644)
	if got := envPort(dir); got != 3004 {
		t.Errorf("envPort = %d", got)
	}
}
