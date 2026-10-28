package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ComposeService is a service of "docker compose config --format json".
type ComposeService struct {
	Image string
	// Environment holds the resolved variables, null values count as empty.
	Environment map[string]string
}

// ComposeConfig reads the services of the compose file in dir. An empty file means the default
// file docker compose finds without -f.
func ComposeConfig(dir, file string) (map[string]ComposeService, error) {
	out, err := run(dir, composeArgs(file, "config", "--format", "json")...)
	if err != nil {
		return nil, err
	}
	return parseComposeConfig([]byte(out))
}

// parseComposeConfig reads the JSON of "docker compose config --format json".
func parseComposeConfig(data []byte) (map[string]ComposeService, error) {
	var raw struct {
		Services map[string]struct {
			Image       string             `json:"image"`
			Environment map[string]*string `json:"environment"`
		} `json:"services"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("docker compose config: %w", err)
	}
	services := map[string]ComposeService{}
	for name, s := range raw.Services {
		env := map[string]string{}
		for k, v := range s.Environment {
			if v != nil {
				env[k] = *v
			} else {
				env[k] = ""
			}
		}
		services[name] = ComposeService{Image: s.Image, Environment: env}
	}
	return services, nil
}

// postgresImages are image names that end with one of these after a "/".
var postgresImages = []string{"postgres", "postgis", "pgvector", "timescaledb"}

// IsPostgres is true for an image named postgres or ending in /postgres, /postgis, /pgvector or
// /timescaledb, without registry and tag, or for a service with a POSTGRES_ variable.
func IsPostgres(s ComposeService) bool {
	for k := range s.Environment {
		if strings.HasPrefix(k, "POSTGRES_") {
			return true
		}
	}
	name := imageName(s.Image)
	if name == "postgres" {
		return true
	}
	for _, img := range postgresImages {
		if strings.HasSuffix(name, "/"+img) {
			return true
		}
	}
	return false
}

// imageName strips registry, tag and digest: "ghcr.io/x/postgis:16@sha256:…" becomes "x/postgis".
func imageName(image string) string {
	image, _, _ = strings.Cut(image, "@")
	if i := strings.LastIndex(image, ":"); i > strings.LastIndex(image, "/") {
		image = image[:i]
	}
	if first, rest, ok := strings.Cut(image, "/"); ok && (strings.ContainsAny(first, ".:") || first == "localhost") {
		image = rest
	}
	return image
}

// RunningContainer is the name of the running container of service whose compose project lives
// in dir. ok is false without one.
func RunningContainer(dir, service string) (name string, ok bool, err error) {
	list, err := all()
	if err != nil {
		return "", false, err
	}
	for _, c := range list {
		if c.State == "running" && c.label(labelWorkingDir) == dir && c.label(labelService) == service {
			return c.Names, true, nil
		}
	}
	return "", false, nil
}

// UpArgs are the arguments of docker for starting service, for the log.
func UpArgs(file, service string) []string {
	return composeArgs(file, "up", "-d", service)
}

// Up starts service of the compose file in dir. An empty file means the default file.
func Up(dir, file, service string) error {
	_, err := run(dir, UpArgs(file, service)...)
	return err
}

// Exec runs args in the container and returns its stdout.
func Exec(container string, args ...string) (string, error) {
	return run("", append([]string{"exec", container}, args...)...)
}

// composeArgs builds "compose [-f file] <args>".
func composeArgs(file string, args ...string) []string {
	out := []string{"compose"}
	if file != "" {
		out = append(out, "-f", file)
	}
	return append(out, args...)
}

// run runs docker in dir. An error names the command and the first line of stderr.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command(Binary, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		what := "docker " + args[0]
		if args[0] == "compose" {
			what = "docker compose"
		}
		if line, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n"); line != "" {
			return "", fmt.Errorf("%s: %s", what, line)
		}
		return "", fmt.Errorf("%s: %w", what, err)
	}
	return string(out), nil
}
