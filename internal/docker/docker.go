// Package docker lists containers through the docker binary. A container belongs to a project
// through the compose label working_dir. Services of the compose file show up as stopped while
// they have no container.
package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Labels Docker Compose writes on every container.
const (
	labelWorkingDir = "com.docker.compose.project.working_dir"
	labelService    = "com.docker.compose.service"
)

// composeFiles are the file names docker compose finds without -f.
var composeFiles = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

// Container is one line of "docker ps --format {{json .}}".
type Container struct {
	Names      string `json:"Names"`
	Image      string `json:"Image"`
	State      string `json:"State"`
	Status     string `json:"Status"`
	Health     string `json:"HealthStatus"`
	RunningFor string `json:"RunningFor"`
	Ports      string `json:"Ports"`
	Labels     string `json:"Labels"`
}

// HostPorts reads the published host ports from "0.0.0.0:5435->5432/tcp, [::]:5435->5432/tcp".
// IPv6 duplicates show up once.
func (c Container) HostPorts() []int {
	var ports []int
	seen := map[int]bool{}
	for part := range strings.SplitSeq(c.Ports, ",") {
		host, _, ok := strings.Cut(strings.TrimSpace(part), "->")
		if !ok {
			continue
		}
		p, err := strconv.Atoi(host[strings.LastIndex(host, ":")+1:])
		if err != nil || seen[p] {
			continue
		}
		seen[p] = true
		ports = append(ports, p)
	}
	return ports
}

// label reads a label from "a=1,b=2". Values containing a comma come back cut off, which is fine
// for working_dir and service.
func (c Container) label(key string) string {
	for part := range strings.SplitSeq(c.Labels, ",") {
		if k, v, ok := strings.Cut(part, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// problem is true for a container that exited with an error, restarts or is unhealthy.
func (c Container) problem() bool {
	switch {
	case c.Health == "unhealthy", c.State == "restarting", c.State == "dead":
		return true
	case c.State == "exited":
		return !strings.HasPrefix(c.Status, "Exited (0)")
	}
	return false
}

// State is the state of a service.
type State int

const (
	Running State = iota
	Stopped
	Failed
)

// Service is a compose service, with its container if one exists.
type Service struct {
	Name      string
	State     State
	Ports     []int
	Container *Container
}

// Info is the docker state of all projects.
type Info struct {
	Containers []Container
	Err        error
	// Services holds the services of the compose file per project path. Projects without a
	// compose file are missing.
	Services map[string][]string
}

// Load reads all containers and the compose services of the projects at paths. Services already
// in known are not read again, because docker compose config takes a noticeable moment per
// project.
func Load(paths []string, known map[string][]string) Info {
	info := Info{Services: map[string][]string{}}
	info.Containers, info.Err = all()
	for _, p := range paths {
		if s, ok := known[p]; ok {
			info.Services[p] = s
		} else if hasCompose(p) {
			info.Services[p], _ = composeServices(p)
		}
	}
	return info
}

// all lists all containers, stopped ones too.
func all() ([]Container, error) {
	cmd := exec.Command("docker", "ps", "-a", "--no-trunc", "--format", "{{json .}}")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("docker ps: %s", msg)
		}
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	var list []Container
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var c Container
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("docker ps: %w", err)
		}
		list = append(list, c)
	}
	return list, nil
}

func hasCompose(dir string) bool {
	for _, f := range composeFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

// composeServices lists the services of the compose file in dir in file order.
func composeServices(dir string) ([]string, error) {
	cmd := exec.Command("docker", "compose", "config", "--services")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// Events reports every container change until ctx ends. It returns nil without docker.
func Events(ctx context.Context) <-chan struct{} {
	cmd := exec.CommandContext(ctx, "docker", "events", "--format", "{{json .}}", "--filter", "type=container")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	ch := make(chan struct{}, 1)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			select {
			case ch <- struct{}{}:
			default:
				// An event is already waiting, one reload covers both.
			}
		}
		cmd.Wait()
	}()
	return ch
}

// ProjectServices lists the services of the project at dir in compose file order, followed by
// containers of the project that the file does not name.
func (d Info) ProjectServices(dir string) []Service {
	var out []Service
	byName := map[string]int{}
	for _, name := range d.Services[dir] {
		byName[name] = len(out)
		out = append(out, Service{Name: name, State: Stopped})
	}
	for i := range d.Containers {
		c := &d.Containers[i]
		if c.label(labelWorkingDir) != dir {
			continue
		}
		name := c.label(labelService)
		if name == "" {
			name = c.Names
		}
		s := Service{Name: name, State: Stopped, Ports: c.HostPorts(), Container: c}
		switch {
		case c.problem():
			s.State = Failed
		case c.State == "running":
			s.State = Running
		}
		if i, ok := byName[s.Name]; ok {
			out[i] = s
			continue
		}
		byName[s.Name] = len(out)
		out = append(out, s)
	}
	return out
}
