package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/devserver"
	"lazydev/internal/docker"
	"lazydev/internal/procnet"
	"lazydev/internal/project"
)

// dockerPollInterval gilt, wenn docker events fehlt, und für die Dev-Zeile.
const dockerPollInterval = 10 * time.Second

// defaultDevPort nutzt Nuxt ohne PORT in der .env.
const defaultDevPort = 3000

type svcState int

const (
	svcRunning svcState = iota
	svcStopped
	svcError
)

type service struct {
	name      string
	state     svcState
	ports     []int
	container *docker.Container
}

type devEntry struct {
	label string
	port  int
}

// dockerInfo ist der Docker- und Dev-Status aller Projekte.
type dockerInfo struct {
	containers []docker.Container
	err        error
	// services hält die Services aus der Compose-Datei pro Projektpfad. Projekte ohne
	// Compose-Datei fehlen.
	services map[string][]string
	dev      map[string][]devEntry
}

type dockerLoadedMsg struct{ info dockerInfo }

type dockerEventMsg struct{}

type dockerTickMsg struct{}

// loadDocker liest Container, Compose-Services und Dev-Server. Compose-Services aus known liest
// loadDocker nicht neu, weil docker compose config pro Projekt spürbar dauert.
func (m Model) loadDocker() tea.Cmd {
	paths := make([]string, len(m.projects))
	for i, p := range m.projects {
		paths[i] = p.Path
	}
	known := m.docker.services
	rs := m.runs
	projects := m.projects
	return func() tea.Msg {
		info := dockerInfo{services: map[string][]string{}, dev: map[string][]devEntry{}}
		info.containers, info.err = docker.All()
		for _, p := range paths {
			if s, ok := known[p]; ok {
				info.services[p] = s
				continue
			}
			if docker.HasCompose(p) {
				s, _ := docker.ComposeServices(p)
				info.services[p] = s
			}
		}
		info.dev = loadDev(projects, rs)
		return dockerLoadedMsg{info: info}
	}
}

// loadDev findet Dev-Server im Hauptordner am Port aus der .env und Dev-Server aus p in Worktrees.
func loadDev(projects []project.Project, rs runs) map[string][]devEntry {
	dev := map[string][]devEntry{}
	mainPorts := map[string]int{}
	var ports []int
	for _, p := range projects {
		port := envPort(p.Path)
		mainPorts[p.Path] = port
		ports = append(ports, port)
	}
	owners := procnet.Owners(ports)
	for _, p := range projects {
		port := mainPorts[p.Path]
		if l, ok := owners[port]; ok && (l.Dir == p.Path || strings.HasPrefix(l.Dir, p.Path+"/")) {
			dev[p.Path] = append(dev[p.Path], devEntry{label: "main", port: port})
		}
		var worktrees []devEntry
		for k, r := range rs {
			if k.projectID == p.ID && devserver.PID(k.projectID, k.prdName) != 0 {
				worktrees = append(worktrees, devEntry{label: k.prdName, port: r.Port})
			}
		}
		sort.Slice(worktrees, func(i, j int) bool { return worktrees[i].port < worktrees[j].port })
		dev[p.Path] = append(dev[p.Path], worktrees...)
	}
	return dev
}

// envPort liest PORT aus der .env des Hauptordners.
func envPort(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		return defaultDevPort
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.TrimSpace(strings.TrimPrefix(k, "export ")) != "PORT" {
			continue
		}
		if p, err := strconv.Atoi(strings.Trim(strings.TrimSpace(v), `"'`)); err == nil {
			return p
		}
	}
	return defaultDevPort
}

// projectServices liefert die Services eines Projekts in der Reihenfolge der Compose-Datei,
// dahinter Container ohne Eintrag in der Datei.
func (d dockerInfo) projectServices(dir string) []service {
	var out []service
	byName := map[string]int{}
	for _, name := range d.services[dir] {
		byName[name] = len(out)
		out = append(out, service{name: name, state: svcStopped})
	}
	for i := range d.containers {
		c := &d.containers[i]
		if c.Label(docker.LabelWorkingDir) != dir {
			continue
		}
		s := service{name: c.Service(), state: svcStopped, ports: c.HostPorts(), container: c}
		switch {
		case c.Problem():
			s.state = svcError
		case c.State == "running":
			s.state = svcRunning
		}
		if i, ok := byName[s.name]; ok {
			out[i] = s
			continue
		}
		byName[s.name] = len(out)
		out = append(out, s)
	}
	return out
}

// projectSymbol: ▲ alle Services laufen, △ einige oder keine, leer ohne Docker.
func (d dockerInfo) projectSymbol(dir string) string {
	services := d.projectServices(dir)
	if len(services) == 0 || d.err != nil {
		return ""
	}
	for _, s := range services {
		if s.state != svcRunning {
			return "△"
		}
	}
	return "▲"
}

func stateSymbol(s svcState) string {
	switch s {
	case svcRunning:
		return styleOK.Render("●")
	case svcError:
		return styleError.Render("✗")
	}
	return styleMuted.Render("○")
}

func portList(ports []int) string {
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = ":" + strconv.Itoa(p)
	}
	return strings.Join(parts, " ")
}

// dockerLine zeigt die Services des Projekts. Passt die Zeile nicht in width, kürzt
// dockerLine auf Anzahl und Ports.
func (d dockerInfo) dockerLine(dir string, width int) (string, bool) {
	label := styleMuted.Render("docker ")
	if d.err != nil {
		return label + styleMuted.Render("– nicht erreichbar"), true
	}
	services := d.projectServices(dir)
	if len(services) == 0 {
		return "", false
	}
	var parts []string
	running := 0
	var ports []int
	for _, s := range services {
		part := stateSymbol(s.state) + " " + s.name
		if len(s.ports) > 0 {
			part += " " + portList(s.ports)
		}
		parts = append(parts, part)
		if s.state == svcRunning {
			running++
			ports = append(ports, s.ports...)
		}
	}
	full := label + strings.Join(parts, "   ")
	if ansi.StringWidth(full) <= width {
		return full, true
	}
	sym := styleOK.Render("●")
	if running < len(services) {
		sym = styleMuted.Render("◐")
	}
	short := fmt.Sprintf("%s%s %d/%d · %s", label, sym, running, len(services), portList(ports))
	return ansi.Truncate(short, width, " …"), true
}

// dockerBlock zeigt jeden Service in einer eigenen Zeile mit Status, Ports, Image und Container.
// Spalten, die nicht in width passen, lässt dockerBlock von rechts weg.
func (d dockerInfo) dockerBlock(dir string, width int) []string {
	if d.err != nil {
		return []string{styleMuted.Render("docker – nicht erreichbar")}
	}
	services := d.projectServices(dir)
	if len(services) == 0 {
		return nil
	}
	nameW, portW := 7, 5
	for _, s := range services {
		nameW = max(nameW, ansi.StringWidth(s.name))
		portW = max(portW, ansi.StringWidth(portList(s.ports)))
	}
	nameW = min(nameW, 20)
	// Breiten von Status, Ports und Image. Der Container bekommt den Rest.
	cols := []int{24, portW, 24}
	used := 2 + nameW
	visible := 0
	for _, w := range cols {
		if used+1+w > width {
			break
		}
		used += 1 + w
		visible++
	}
	titles := []string{"Status", "Ports", "Image"}
	header := "  " + padRight("Docker", nameW)
	for i := 0; i < visible; i++ {
		header += " " + padRight(titles[i], cols[i])
	}
	if visible == len(cols) && width-used > 8 {
		header += " Container"
	}
	lines := []string{styleMuted.Render(header)}
	for _, s := range services {
		status, image, name := "gestoppt", "–", "–"
		if c := s.container; c != nil {
			status, image, name = c.Status, c.Image, c.Names
		}
		cells := []string{status, portList(s.ports), image}
		row := stateSymbol(s.state) + " " + padRight(s.name, nameW)
		for i := 0; i < visible; i++ {
			row += " " + padRight(cells[i], cols[i])
		}
		if visible == len(cols) && width-used > 8 {
			row += " " + styleMuted.Render(ansi.Truncate(name, width-used-1, "…"))
		}
		lines = append(lines, row)
	}
	return lines
}

func (d dockerInfo) devLine(dir string) (string, bool) {
	entries := d.dev[dir]
	if len(entries) == 0 {
		return "", false
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = fmt.Sprintf("%s %s :%d", styleOK.Render("●"), e.label, e.port)
	}
	return styleMuted.Render("dev    ") + strings.Join(parts, "   "), true
}

// dockerEvents startet docker events. Ohne Docker liefert dockerEvents nil, und der Tick reicht.
func dockerEvents() <-chan struct{} {
	ch, err := docker.Events(context.Background())
	if err != nil {
		return nil
	}
	return ch
}

func waitDockerEvent(ch <-chan struct{}) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		if _, ok := <-ch; !ok {
			return nil
		}
		return dockerEventMsg{}
	}
}

func dockerTick() tea.Cmd {
	return tea.Tick(dockerPollInterval, func(time.Time) tea.Msg { return dockerTickMsg{} })
}

// worktreeDatabases listet pro Postgres-Container die Datenbanken laufender PRDs des Projekts.
// Ein Name gehört zu einem PRD, wenn er auf "_<slug>" endet.
func worktreeDatabases(containers []docker.Container, slugs map[string]string) map[string][]string {
	out := map[string][]string{}
	for _, c := range containers {
		if c.State != "running" || !strings.HasPrefix(c.Image, "postgres") {
			continue
		}
		user, err := docker.Exec(c.Names, "printenv", "POSTGRES_USER")
		user = strings.TrimSpace(user)
		if err != nil || user == "" {
			user = "postgres"
		}
		names, err := docker.Exec(c.Names, "psql", "-U", user, "-d", "postgres", "-tAc", "SELECT datname FROM pg_database")
		if err != nil {
			continue
		}
		for _, n := range strings.Split(strings.TrimSpace(names), "\n") {
			for slug, prd := range slugs {
				if strings.HasSuffix(n, "_"+slug) {
					out[c.Names] = append(out[c.Names], n+" → "+prd)
				}
			}
		}
		slices.Sort(out[c.Names])
	}
	return out
}
