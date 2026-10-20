package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Compose-Labels, die Docker Compose an jeden Container schreibt.
const (
	LabelWorkingDir = "com.docker.compose.project.working_dir"
	LabelService    = "com.docker.compose.service"
)

// composeFiles sind die Dateinamen, die docker compose ohne -f findet.
var composeFiles = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

// Label liest ein Label aus "a=1,b=2". Werte mit Komma, etwa config_files, liefert Label
// unvollständig. lazydev braucht nur working_dir und service.
func (c Container) Label(key string) string {
	for _, part := range strings.Split(c.Labels, ",") {
		if k, v, ok := strings.Cut(part, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// Service liefert den Compose-Service, ohne Compose den Containernamen.
func (c Container) Service() string {
	if s := c.Label(LabelService); s != "" {
		return s
	}
	return c.Names
}

// Problem meldet einen Container, der mit Fehler endete, neu startet oder unhealthy ist.
func (c Container) Problem() bool {
	switch {
	case c.Health == "unhealthy", c.State == "restarting", c.State == "dead":
		return true
	case c.State == "exited":
		return !strings.HasPrefix(c.Status, "Exited (0)")
	}
	return false
}

// All liefert alle Container, auch gestoppte.
func All() ([]Container, error) {
	out, err := run("ps", "-a", "--no-trunc", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	return parseList(out)
}

func parseList(out string) ([]Container, error) {
	var list []Container
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var c Container
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

// HasCompose meldet, ob dir eine Compose-Datei enthält.
func HasCompose(dir string) bool {
	for _, f := range composeFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

// ComposeServices liefert die Services aus der Compose-Datei in dir.
func ComposeServices(dir string) ([]string, error) {
	cmd := exec.Command("docker", "compose", "config", "--services")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var services []string
	for _, s := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if s != "" {
			services = append(services, s)
		}
	}
	return services, nil
}

// Events meldet jede Änderung an einem Container, bis ctx endet oder der Strom abbricht.
// Der Kanal schließt am Ende.
func Events(ctx context.Context) (<-chan struct{}, error) {
	cmd := exec.CommandContext(ctx, "docker", "events", "--format", "{{json .}}", "--filter", "type=container")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	ch := make(chan struct{}, 1)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			select {
			case ch <- struct{}{}:
			default:
				// Ein Ereignis wartet schon, ein Neueinlesen reicht für beide.
			}
		}
		cmd.Wait()
	}()
	return ch, nil
}
