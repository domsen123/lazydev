package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/docker"
)

type databasesLoadedMsg struct {
	project string
	dbs     map[string][]string
}

// openDocker zeigt die Docker-Ansicht des gewählten Projekts und lädt die Worktree-Datenbanken.
func (m Model) openDocker() (Model, tea.Cmd) {
	p := m.selectedProject()
	if p == nil {
		return m, nil
	}
	m.mode = modeDocker
	m.dockerCursor = 0
	m.dockerDBs = nil
	var containers []docker.Container
	for _, s := range m.docker.projectServices(p.Path) {
		if s.container != nil {
			containers = append(containers, *s.container)
		}
	}
	slugs := map[string]string{}
	for k, r := range m.runs {
		if k.projectID == p.ID && r.Slug != "" {
			slugs[r.Slug] = k.prdName
		}
	}
	path := p.Path
	return m, func() tea.Msg {
		return databasesLoadedMsg{project: path, dbs: worktreeDatabases(containers, slugs)}
	}
}

func (m Model) updateDocker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.selectedProject()
	if p == nil {
		m.mode = modeNormal
		return m, nil
	}
	services := m.docker.projectServices(p.Path)
	switch msg.String() {
	case "esc", "q", "D":
		m.mode = modeNormal
	case "j", "down":
		m.dockerCursor = clamp(m.dockerCursor+1, len(services))
	case "k", "up":
		m.dockerCursor = clamp(m.dockerCursor-1, len(services))
	case "U", "S":
		if !docker.HasCompose(p.Path) {
			m.setMessage("Keine Compose-Datei im Hauptordner", true)
			return m, nil
		}
		args, verb := []string{"compose", "up", "-d"}, "starten"
		if msg.String() == "S" {
			args, verb = []string{"compose", "stop"}, "stoppen"
		}
		dir, name := p.Path, p.Name()
		next, cmd := m.ask(fmt.Sprintf("Container von %s %s (docker %s)? (j/n)", name, verb, strings.Join(args, " ")), func(m Model) (Model, tea.Cmd) {
			m.setMessage("docker "+strings.Join(args, " ")+" …", false)
			return m, func() tea.Msg {
				c := exec.Command("docker", args...)
				c.Dir = dir
				out, err := c.CombinedOutput()
				if err != nil {
					return actionDoneMsg{err: fmt.Errorf("docker %s: %s", strings.Join(args, " "), firstLine(strings.TrimSpace(string(out))))}
				}
				return actionDoneMsg{text: "docker " + strings.Join(args, " ") + " fertig"}
			}
		})
		// Nach der Rückfrage geht es in der Docker-Ansicht weiter.
		next.returnMode = modeDocker
		return next, cmd
	case "L":
		if m.dockerCursor >= len(services) || services[m.dockerCursor].container == nil {
			m.setMessage("Für diesen Service gibt es keinen Container", true)
			return m, nil
		}
		name := services[m.dockerCursor].container.Names
		return m.openInteractive(p.Path, []string{"docker", "logs", "-f", "--tail", "200", name}, "Logs von "+name)
	}
	return m, nil
}

func (m Model) viewDocker() string {
	p := m.selectedProject()
	if p == nil {
		return ""
	}
	services := m.docker.projectServices(p.Path)
	var lines []string
	if m.docker.err != nil {
		lines = append(lines, styleError.Render(" Docker nicht erreichbar: "+firstLine(m.docker.err.Error())))
	}
	if len(services) == 0 {
		lines = append(lines, styleMuted.Render(" Keine Container und keine Compose-Datei"))
	}
	lines = append(lines, styleMuted.Render("    "+padRight("Service", 18)+" "+padRight("Container", 30)+" "+
		padRight("Status", 24)+" "+padRight("Ports", 20)+" "+padRight("Image", 20)+" Seit"))
	for i, s := range services {
		name, status, image, since := "–", "gestoppt", "–", ""
		if c := s.container; c != nil {
			name, status, image, since = c.Names, c.Status, c.Image, c.RunningFor
		}
		row := stateSymbol(s.state) + " " + padRight(s.name, 18) + " " + padRight(name, 30) + " " +
			padRight(status, 24) + " " + padRight(portList(s.ports), 20) + " " + padRight(image, 20) + " " + since
		lines = append(lines, cursorLine(i == m.dockerCursor, true, row))
		if s.container != nil {
			for _, db := range m.dockerDBs[s.container.Names] {
				lines = append(lines, styleMuted.Render("      └ "+db))
			}
		}
	}
	help := " " + strings.Join([]string{
		styleKey.Render("U") + " compose up -d", styleKey.Render("S") + " compose stop",
		styleKey.Render("L") + " Logs", styleKey.Render("Esc") + " zurück",
	}, styleMuted.Render(" · "))
	status := ""
	switch {
	case m.mode == modeConfirm && m.confirm != nil:
		status = " " + styleSelected.Render(m.confirm.question)
	case m.message != "" && m.messageErr:
		status = styleError.Render(" " + m.message)
	case m.message != "":
		status = styleOK.Render(" " + m.message)
	}
	return box("Docker: "+p.Name(), lines, m.width, m.height-2, true) + "\n" + status + "\n" + help
}

// interruptSafe hält lazydev am Leben, wenn Ctrl+C einen Befehl im selben Terminal beendet. Das
// Signal trifft die ganze Vordergrund-Prozessgruppe, also auch lazydev. signal.Notify fängt es
// ab. Anders als ein ignoriertes Signal erbt der Kindprozess das Abfangen nicht.
type interruptSafe struct{ *exec.Cmd }

func (c interruptSafe) Run() error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	defer signal.Stop(ch)
	return c.Cmd.Run()
}

func (c interruptSafe) SetStdin(r io.Reader)  { c.Cmd.Stdin = r }
func (c interruptSafe) SetStdout(w io.Writer) { c.Cmd.Stdout = w }
func (c interruptSafe) SetStderr(w io.Writer) { c.Cmd.Stderr = w }
