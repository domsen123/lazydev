package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/notify"
)

// The tabs of the left menu column.
const (
	tabGeneral = iota
	tabDiscord
	tabGit
	tabQuit
	menuTabs
)

var menuTabNames = [menuTabs]string{"General", "Discord", "Git", "Quit"}

// menuLeftWidth is the inner width of the left menu column.
const menuLeftWidth = 14

// menuMaxWidth is the outer width of the menu box on a wide screen.
const menuMaxWidth = 72

// The entries of the right column.
type menuItem int

const (
	itemTheme menuItem = iota
	itemNotifications
	itemGlobalURL
	itemProject
	itemProjectURL
	itemSendTest
)

// URL targets of the input.
const (
	urlGlobal  = "global"
	urlProject = "project"
)

// tabDescriptions describe the tabs without entries and the General tab below its entry.
var tabDescriptions = [menuTabs]string{
	tabGeneral: "Colors of borders, focus and selection. lazydev saves the choice in settings.json.",
	tabGit:     "Fetch, pull, push and commit the main folder of the selected project. Each action runs in a center tab. Also alt+g.",
	tabQuit:    "Ends lazydev and its terminal tabs. Runners keep working in the background.",
}

// itemDescriptions describe the entries of the Discord tab.
var itemDescriptions = map[menuItem]string{
	itemNotifications: "Runners post to Discord when a PRD is merged, fails, gets aborted or waits for the main folder.",
	itemGlobalURL:     "Webhook URL for all projects. Stored in settings.json, never in a repository.",
	itemProject:       "Off mutes this project only.",
	itemProjectURL:    "Replaces the global URL for this project. Empty uses the global URL.",
	itemSendTest:      "Posts a test message to the URL of this project now, also while off.",
}

// menu is the btop style menu that alt+m opens: tabs on the left, their entries or a description on
// the right. Every change applies and saves right away.
type menu struct {
	tab int
	// right is true while the keys move in the right column.
	right  bool
	cursor int
	// editing is the target of the open URL input, urlGlobal or urlProject, empty while closed.
	editing string
	input   textinput.Model
}

// discordTestMsg is the outcome of Send test.
type discordTestMsg struct{ err error }

// menuItems lists the entries of the right column of the open tab. The Discord tab has the project
// entries only with a selected project.
func (m model) menuItems() []menuItem {
	switch m.menu.tab {
	case tabGeneral:
		return []menuItem{itemTheme}
	case tabDiscord:
		if m.current() == nil {
			return []menuItem{itemNotifications, itemGlobalURL, itemSendTest}
		}
		return []menuItem{itemNotifications, itemGlobalURL, itemProject, itemProjectURL, itemSendTest}
	}
	return nil
}

// selectedItem is the entry under the cursor of the right column, -1 without entries.
func (m model) selectedItem() menuItem {
	items := m.menuItems()
	if len(items) == 0 {
		return -1
	}
	return items[clamp(m.menu.cursor, len(items))]
}

// updateMenu handles all keys while the menu is open.
func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.menu.editing != "":
		return m.updateMenuInput(msg)
	case m.menu.right:
		return m.updateMenuRight(msg)
	}
	switch msg.String() {
	case "esc", "q", "alt+m":
		m.menu = nil
	case "up", "k":
		m.menu.tab = (m.menu.tab + menuTabs - 1) % menuTabs
	case "down", "j", "tab":
		m.menu.tab = (m.menu.tab + 1) % menuTabs
	case "enter", "right", "l":
		switch m.menu.tab {
		case tabGeneral, tabDiscord:
			m.menu.right, m.menu.cursor = true, 0
		case tabGit:
			m.menu = nil
			m.openCockpit()
		case tabQuit:
			return m.quit()
		}
	}
	return m, nil
}

// updateMenuRight handles the keys in the right column.
func (m model) updateMenuRight(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.menuItems())
	switch msg.String() {
	case "esc", "h":
		m.menu.right = false
	case "q", "alt+m":
		m.menu = nil
	case "up", "k":
		m.menu.cursor = (clamp(m.menu.cursor, n) + n - 1) % n
	case "down", "j", "tab":
		m.menu.cursor = (clamp(m.menu.cursor, n) + 1) % n
	case "left":
		m.changeItem(-1)
	case "right":
		m.changeItem(1)
	case "enter", " ":
		switch item := m.selectedItem(); item {
		case itemGlobalURL, itemProjectURL:
			target := urlGlobal
			if item == itemProjectURL {
				target = urlProject
			}
			in := textinput.New()
			in.Prompt = "Webhook URL: "
			m.menu.editing, m.menu.input = target, in
			return m, m.menu.input.Focus()
		case itemSendTest:
			return m, m.sendTest()
		default:
			m.changeItem(1)
		}
	}
	return m, nil
}

// changeItem moves the theme by step or flips the switch under the cursor.
func (m *model) changeItem(step int) {
	p := m.current()
	switch m.selectedItem() {
	case itemTheme:
		m.setTheme(m.theme + step)
	case itemNotifications:
		m.saveDiscord(func(d *config.Discord) { d.Enabled = !d.Enabled })
	case itemProject:
		m.saveDiscord(func(d *config.Discord) {
			on := !d.ProjectEnabled(p.path)
			setDiscordProject(d, p.path, func(dp *config.DiscordProject) { dp.Enabled = &on })
		})
	}
}

// updateMenuInput handles the URL input. An empty URL removes it, anything else must be https.
func (m model) updateMenuInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.menu.editing = ""
		m.menu.input.Blur()
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.menu.input.Value())
		if url != "" && !strings.HasPrefix(url, "https://") {
			m.err = "Enter an https URL."
			return m, nil
		}
		if m.menu.editing == urlGlobal {
			m.saveDiscord(func(d *config.Discord) { d.URL = url })
		} else if p := m.current(); p != nil {
			m.saveDiscord(func(d *config.Discord) {
				setDiscordProject(d, p.path, func(dp *config.DiscordProject) { dp.URL = url })
			})
		}
		m.menu.editing = ""
		m.menu.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.menu.input, cmd = m.menu.input.Update(msg)
	return m, cmd
}

// saveDiscord reads settings.json, applies change to its Discord settings and writes it back.
func (m *model) saveDiscord(change func(*config.Discord)) {
	s := config.LoadSettings()
	change(&s.Discord)
	if err := config.SaveSettings(s); err != nil {
		m.err = err.Error()
	}
}

// setDiscordProject applies change to the entry of the project in dir. An entry without settings
// drops out.
func setDiscordProject(d *config.Discord, dir string, change func(*config.DiscordProject)) {
	p := d.Projects[dir]
	change(&p)
	if p == (config.DiscordProject{}) {
		delete(d.Projects, dir)
		return
	}
	if d.Projects == nil {
		d.Projects = map[string]config.DiscordProject{}
	}
	d.Projects[dir] = p
}

// sendTest posts the test message once to the URL of the selected project, the global URL without
// project. It ignores the switches.
func (m *model) sendTest() tea.Cmd {
	d := config.LoadSettings().Discord
	url, name := d.URL, ""
	if p := m.current(); p != nil {
		url, name = d.ProjectURL(p.path), p.name()
	}
	if url == "" {
		m.err = "No webhook URL set."
		return nil
	}
	return func() tea.Msg {
		body, err := notify.TestPayload(name)
		if err == nil {
			err = notify.Post(context.Background(), url, body)
		}
		return discordTestMsg{err: err}
	}
}

// setTheme switches to theme i, wrapping around, and saves the choice for the next start.
func (m *model) setTheme(i int) {
	n := len(themes)
	m.theme = (i%n + n) % n
	if err := saveTheme(m.theme); err != nil {
		m.err = err.Error()
	}
}

// shortURL shows a webhook URL as "…/" and its last 4 characters, so the secret stays hidden.
func shortURL(url string) string {
	r := []rune(url)
	return "…/" + string(r[max(0, len(r)-4):])
}

// onOff renders a switch.
func onOff(on bool) string {
	if on {
		return "‹ on ›"
	}
	return "‹ off ›"
}

// itemLabel is the text of an entry of the right column.
func (m model) itemLabel(item menuItem, d config.Discord) string {
	p := m.current()
	switch item {
	case itemTheme:
		return "Color theme  ‹ " + themes[m.theme].name + " ›"
	case itemNotifications:
		return "Notifications  " + onOff(d.Enabled)
	case itemGlobalURL:
		url := "not set"
		if d.URL != "" {
			url = shortURL(d.URL)
		}
		return "Global URL  " + url
	case itemProject:
		return p.name() + "  " + onOff(d.ProjectEnabled(p.path))
	case itemProjectURL:
		url := "global"
		if u := d.Projects[p.path].URL; u != "" {
			url = shortURL(u)
		}
		return "URL for " + p.name() + "  " + url
	case itemSendTest:
		return "Send test"
	}
	return ""
}

// keys is the hint line of the menu.
func (mn *menu) keys() string {
	switch {
	case mn.editing != "":
		return "enter save · esc cancel"
	case mn.right:
		return "↑/↓ select · ←/→ change · enter edit · esc back"
	}
	return "↑/↓ select · enter open · esc close"
}

// viewMenu renders the menu box: tabs left, entries or description right, the hint line below.
func (m model) viewMenu() string {
	t := themes[m.theme]
	width := min(m.width-4, menuMaxWidth)
	inner := width - 2
	rightWidth := max(1, inner-menuLeftWidth-2)
	text := lipgloss.NewStyle().Foreground(t.text)
	selected := text.Bold(true).Foreground(t.selectedFg).Background(t.selectedBg)

	blank := strings.Repeat(" ", menuLeftWidth)
	left := []string{blank}
	for i, name := range menuTabNames {
		if i == tabQuit {
			left = append(left, blank)
		}
		cell := pad(" "+name, menuLeftWidth)
		switch {
		case i == m.menu.tab && !m.menu.right:
			left = append(left, selected.Render(cell))
		case i == m.menu.tab:
			left = append(left, text.Bold(true).Foreground(t.focus).Render(cell))
		default:
			left = append(left, text.Render(cell))
		}
	}

	right := []string{""}
	description := tabDescriptions[m.menu.tab]
	items := m.menuItems()
	if len(items) > 0 {
		d := config.LoadSettings().Discord
		cursor := clamp(m.menu.cursor, len(items))
		if m.menu.editing != "" {
			in := m.menu.input
			in.PromptStyle = text.Bold(true)
			in.TextStyle = text
			in.Width = max(1, rightWidth-lipgloss.Width(in.Prompt)-1)
			right = append(right, ansi.Truncate(in.View(), rightWidth, ""))
		} else {
			for i, item := range items {
				cell := pad(m.itemLabel(item, d), rightWidth)
				if i == cursor && m.menu.right {
					right = append(right, selected.Render(cell))
				} else {
					right = append(right, text.Render(cell))
				}
			}
		}
		if m.menu.tab == tabDiscord {
			description = itemDescriptions[items[cursor]]
		}
		right = append(right, "")
	}
	wrapped := lipgloss.NewStyle().Width(rightWidth).Render(description)
	for _, line := range strings.Split(wrapped, "\n") {
		right = append(right, text.Faint(true).Render(pad(line, rightWidth)))
	}
	if m.menu.tab == tabGeneral {
		right = append(right, "")
		wrapped := lipgloss.NewStyle().Width(rightWidth).Render(m.versionLine())
		for _, line := range strings.Split(wrapped, "\n") {
			right = append(right, text.Render(pad(line, rightWidth)))
		}
	}

	divider := lipgloss.NewStyle().Foreground(t.divider).Render("│")
	var lines []string
	for i := range max(len(left), len(right)) {
		l, r := blank, strings.Repeat(" ", rightWidth)
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, l+divider+" "+r)
	}
	hint := text.Width(inner).Align(lipgloss.Center)
	lines = append(lines, "", hint.Render(ansi.Truncate(m.menu.keys(), inner, "…")))
	return box("Menu", strings.Join(lines, "\n"), width, len(lines)+2, t.focus, true, t)
}

// overlay draws fg centered on top of bg. Both are multi line strings with ANSI styles.
func overlay(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	fgWidth := 0
	for _, l := range fgLines {
		fgWidth = max(fgWidth, ansi.StringWidth(l))
	}
	x := max(0, (width-fgWidth)/2)
	y := max(0, (height-len(fgLines))/2)
	for i, l := range fgLines {
		if y+i >= len(bgLines) {
			break
		}
		line := bgLines[y+i]
		left := ansi.Truncate(line, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		bgLines[y+i] = left + "\x1b[0m" + l + "\x1b[0m" + ansi.TruncateLeft(line, x+ansi.StringWidth(l), "")
	}
	return strings.Join(bgLines, "\n")
}
