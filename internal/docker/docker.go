// Package docker fragt Container über das docker-Binärprogramm ab.
package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Container struct {
	ID         string `json:"ID"`
	Names      string `json:"Names"`
	Image      string `json:"Image"`
	State      string `json:"State"`
	Status     string `json:"Status"`
	Health     string `json:"HealthStatus"`
	RunningFor string `json:"RunningFor"`
	Ports      string `json:"Ports"`
	Labels     string `json:"Labels"`
}

// HostPorts liest die veröffentlichten Host-Ports aus "0.0.0.0:5435->5432/tcp, [::]:5435->5432/tcp".
// IPv6-Duplikate erscheinen nur einmal.
func (c Container) HostPorts() []int {
	var ports []int
	seen := map[int]bool{}
	for _, part := range strings.Split(c.Ports, ",") {
		host, _, ok := strings.Cut(strings.TrimSpace(part), "->")
		if !ok {
			continue
		}
		i := strings.LastIndex(host, ":")
		p, err := strconv.Atoi(host[i+1:])
		if err != nil || seen[p] {
			continue
		}
		seen[p] = true
		ports = append(ports, p)
	}
	return ports
}

// Running liefert alle laufenden Container.
func Running() ([]Container, error) {
	out, err := run("ps", "--no-trunc", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	list, err := parseList(out)
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	return list, nil
}

// ByHostPort findet den laufenden Container, der den Host-Port veröffentlicht.
// ok ist false, wenn keiner läuft.
func ByHostPort(port int) (c Container, ok bool, err error) {
	list, err := Running()
	if err != nil {
		return Container{}, false, err
	}
	for _, c := range list {
		for _, p := range c.HostPorts() {
			if p == port {
				return c, true, nil
			}
		}
	}
	return Container{}, false, nil
}

// Exec führt einen Befehl im Container aus und liefert stdout.
func Exec(container string, args ...string) (string, error) {
	return run(append([]string{"exec", container}, args...)...)
}

func run(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("docker %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}
