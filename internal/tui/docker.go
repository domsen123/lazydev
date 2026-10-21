package tui

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/docker"
)

// The docker tab shows the containers of the selected project. They refresh on docker events and
// with a tick, for when docker events is missing or misses something.

// dockerInterval is the fallback refresh.
const dockerInterval = 10 * time.Second

type dockerLoadedMsg struct{ info docker.Info }

type dockerEventMsg struct{}

type dockerTickMsg struct{}

func dockerTick() tea.Cmd {
	return tea.Tick(dockerInterval, func(time.Time) tea.Msg { return dockerTickMsg{} })
}

// loadDocker reads containers and compose services in the background. Services in known are not
// read again.
func loadDocker(projects []*projectEntry, known map[string][]string) tea.Cmd {
	paths := projectPaths(projects)
	return func() tea.Msg { return dockerLoadedMsg{info: docker.Load(paths, known)} }
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

func portList(ports []int) string {
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = ":" + strconv.Itoa(p)
	}
	return strings.Join(parts, " ")
}

// dockerView renders the services of p as a table of height rows: state, service, status, ports,
// image and age. Columns that do not fit into width drop from the right. While focused, the cursor
// row gets a background.
func dockerView(d docker.Info, p *projectEntry, width, height int, focused bool, t theme) string {
	text := lipgloss.NewStyle().Foreground(t.text)
	if d.Err != nil {
		msg, _, _ := strings.Cut(d.Err.Error(), "\n")
		return text.Faint(true).Render(ansi.Truncate("Docker not reachable: "+msg, width, "…"))
	}
	services := d.ProjectServices(p.path)
	if len(services) == 0 {
		return text.Faint(true).Render("No containers and no compose file.")
	}
	p.dockerCursor = clamp(p.dockerCursor, len(services))

	nameW, portW := 7, 5
	for _, s := range services {
		nameW = max(nameW, ansi.StringWidth(s.Name))
		portW = max(portW, ansi.StringWidth(portList(s.Ports)))
	}
	nameW = min(nameW, 20)
	titles := []string{"Status", "Ports", "Image", "Created"}
	widths := []int{24, portW, 24, 14}
	used := 3 + nameW
	visible := 0
	for _, w := range widths {
		if used+1+w > width {
			break
		}
		used += 1 + w
		visible++
	}
	// row is a line without the leading " ● ", which gets its own color.
	inner := max(0, width-3)
	row := func(name string, cells []string) string {
		line := pad(name, nameW)
		for i := range visible {
			line += " " + pad(cells[i], widths[i])
		}
		return pad(line, inner)
	}
	lines := []string{text.Faint(true).Render(ansi.Truncate("   "+row("Service", titles), width, ""))}

	// Keep the cursor visible below the header.
	rows := max(1, height-1)
	top := max(0, min(p.dockerCursor-rows+1, len(services)-rows))
	for i := top; i < min(top+rows, len(services)); i++ {
		s := services[i]
		status, image, since := "stopped", "–", ""
		if c := s.Container; c != nil {
			status, image, since = c.Status, c.Image, c.RunningFor
		}
		symbol, color := "○", t.text
		switch s.State {
		case docker.Running:
			symbol, color = "●", colorOK
		case docker.Failed:
			symbol, color = "✗", t.focus
		}
		style := text
		if i == p.dockerCursor && focused {
			style = style.Foreground(t.selectedFg).Background(t.selectedBg)
		} else if s.State == docker.Stopped {
			style = style.Faint(true)
		}
		line := style.Render(" ") + style.Foreground(color).Render(symbol) +
			style.Render(" "+row(s.Name, []string{status, portList(s.Ports), image, since}))
		lines = append(lines, ansi.Truncate(line, width, ""))
	}
	return strings.Join(lines, "\n")
}

// pad cuts or fills s to exactly w cells.
func pad(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
