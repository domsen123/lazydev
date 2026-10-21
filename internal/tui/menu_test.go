package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
)

var (
	altM     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m"), Alt: true}
	keyRight = tea.KeyMsg{Type: tea.KeyRight}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
)

// menuModel is cockpitModel with the menu open.
func menuModel(t *testing.T) model {
	t.Helper()
	m := press(t, cockpitModel(t), altM)
	if m.menu == nil {
		t.Fatal("alt+m opened no menu")
	}
	return m
}

// discordModel has the right column of the Discord tab open.
func discordModel(t *testing.T) model {
	t.Helper()
	return press(t, menuModel(t), keyRunes("j"), keyEnter)
}

// moveTo moves the cursor of the right column to item.
func moveTo(t *testing.T, m model, item menuItem) model {
	t.Helper()
	for range 10 {
		if m.selectedItem() == item {
			return m
		}
		m = press(t, m, keyDown)
	}
	t.Fatalf("no item %d in %v", item, m.menuItems())
	return m
}

func typeText(t *testing.T, m model, s string) model {
	t.Helper()
	return press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func TestMenuView(t *testing.T) {
	m := menuModel(t)
	view := m.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"Menu", "General", "Discord", "Git", "Quit", "Color theme  ‹ Default ›",
		"Colors of borders, focus and selection.", "↑/↓ select · enter open · esc close"} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	checkSize(t, view, 160, 40)
	m = press(t, m, keyRunes("j"), keyRunes("j"))
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "Fetch, pull, push and commit the main folder") {
		t.Errorf("Git tab:\n%s", plain)
	}
	m = press(t, m, keyRunes("j"))
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "Ends lazydev and its terminal tabs.") {
		t.Errorf("Quit tab:\n%s", plain)
	}
	// The tabs wrap around.
	if m = press(t, m, keyRunes("j")); m.menu.tab != tabGeneral {
		t.Errorf("tab after Quit: %d", m.menu.tab)
	}
	if m = press(t, m, keyRunes("k")); m.menu.tab != tabQuit {
		t.Errorf("tab before General: %d", m.menu.tab)
	}
}

func TestMenuNarrow(t *testing.T) {
	m := menuModel(t)
	m.width = 60
	m = press(t, m, keyRunes("j"), keyEnter)
	checkSize(t, m.View(), 60, 40)
}

func TestMenuGitOpensCockpit(t *testing.T) {
	m := press(t, menuModel(t), keyRunes("j"), keyRunes("j"), keyEnter)
	if m.menu != nil || m.cockpit == nil {
		t.Errorf("menu %v, cockpit %v, err %q", m.menu, m.cockpit, m.err)
	}
}

func TestMenuTheme(t *testing.T) {
	m := press(t, menuModel(t), keyRight)
	if m.menu == nil || !m.menu.right {
		t.Fatal("→ did not open the right column")
	}
	m = press(t, m, keyRight)
	if m.theme != 1 {
		t.Errorf("theme: %d", m.theme)
	}
	if s := config.LoadSettings(); s.Theme != themes[1].name {
		t.Errorf("saved theme: %q", s.Theme)
	}
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "↑/↓ select · ←/→ change · enter edit · esc back") {
		t.Errorf("hint:\n%s", plain)
	}
}

func TestMenuNotificationsSwitch(t *testing.T) {
	m := press(t, discordModel(t), keyEnter)
	if !config.LoadSettings().Discord.Enabled {
		t.Error("enabled not saved")
	}
	data, _ := os.ReadFile(filepath.Join(config.Dir(), "settings.json"))
	if !strings.Contains(string(data), `"enabled": true`) {
		t.Errorf("settings.json:\n%s", data)
	}
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "Notifications  ‹ on ›") || !strings.Contains(plain, "Runners post to Discord") {
		t.Errorf("view:\n%s", plain)
	}
	press(t, m, keyRight)
	if config.LoadSettings().Discord.Enabled {
		t.Error("→ did not switch off")
	}
}

func TestMenuProjectSwitch(t *testing.T) {
	m := moveTo(t, discordModel(t), itemProject)
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "shop  ‹ on ›") || !strings.Contains(plain, "Off mutes this project only.") {
		t.Errorf("view:\n%s", plain)
	}
	press(t, m, keyEnter)
	d := config.LoadSettings().Discord
	if p := d.Projects[m.projects[0].path]; p.Enabled == nil || *p.Enabled {
		t.Errorf("project entry: %+v", d.Projects)
	}
}

func TestMenuGlobalURL(t *testing.T) {
	m := press(t, moveTo(t, discordModel(t), itemGlobalURL), keyEnter)
	if m.menu.editing != urlGlobal {
		t.Fatalf("enter opened no input: %+v", m.menu)
	}
	m = press(t, typeText(t, m, "http://x"), keyEnter)
	if m.err != "Enter an https URL." || m.menu.editing == "" {
		t.Errorf("err %q, editing %q", m.err, m.menu.editing)
	}
	if u := config.LoadSettings().Discord.URL; u != "" {
		t.Errorf("saved %q", u)
	}
	m = press(t, m, keyEsc, keyEnter)
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "Webhook URL: ") || !strings.Contains(plain, "enter save · esc cancel") {
		t.Errorf("input view:\n%s", plain)
	}
	m = press(t, typeText(t, m, " https://discord.com/api/webhooks/1/abcd "), keyEnter)
	if m.menu.editing != "" || m.err != "" {
		t.Errorf("editing %q, err %q", m.menu.editing, m.err)
	}
	if u := config.LoadSettings().Discord.URL; u != "https://discord.com/api/webhooks/1/abcd" {
		t.Errorf("saved %q", u)
	}
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Global URL  …/abcd") || strings.Contains(plain, "webhooks") {
		t.Errorf("view:\n%s", plain)
	}
	// An empty input removes the URL.
	m = press(t, m, keyEnter, keyEnter)
	if u := config.LoadSettings().Discord.URL; u != "" {
		t.Errorf("not removed: %q", u)
	}
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "Global URL  not set") {
		t.Errorf("view:\n%s", plain)
	}
}

func TestMenuProjectURL(t *testing.T) {
	m := moveTo(t, discordModel(t), itemProjectURL)
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "URL for shop  global") {
		t.Errorf("view:\n%s", plain)
	}
	m = press(t, typeText(t, press(t, m, keyEnter), "https://x/wxyz"), keyEnter)
	if u := config.LoadSettings().Discord.Projects[m.projects[0].path].URL; u != "https://x/wxyz" {
		t.Errorf("saved %q", u)
	}
	if plain := ansi.Strip(m.View()); !strings.Contains(plain, "URL for shop  …/wxyz") {
		t.Errorf("view:\n%s", plain)
	}
}

func TestMenuWithoutProject(t *testing.T) {
	isolateTUI(t)
	m := model{width: 160, height: 40, focus: colLeft, gitTabs: map[int]string{}}
	m = press(t, m, altM, keyRunes("j"), keyEnter)
	if items := m.menuItems(); len(items) != 3 {
		t.Errorf("items: %v", items)
	}
	m = press(t, moveTo(t, m, itemSendTest), keyEnter)
	if m.err != "No webhook URL set." {
		t.Errorf("err: %q", m.err)
	}
}

func TestMenuSendTest(t *testing.T) {
	m := moveTo(t, discordModel(t), itemSendTest)
	if m = press(t, m, keyEnter); m.err != "No webhook URL set." {
		t.Errorf("err: %q", m.err)
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	// The switches are off, the test goes out anyway.
	config.SaveSettings(config.Settings{Discord: config.Discord{URL: srv.URL}})
	next, cmd := m.Update(keyEnter)
	if cmd == nil {
		t.Fatalf("no command, err %q", next.(model).err)
	}
	next, _ = next.Update(cmd())
	if m = next.(model); m.note != "discord: test sent" || requests != 1 {
		t.Errorf("note %q, err %q, requests %d", m.note, m.err, requests)
	}
	next, _ = m.Update(discordTestMsg{err: os.ErrDeadlineExceeded})
	if m = next.(model); !strings.HasPrefix(m.err, "discord: ") {
		t.Errorf("err: %q", m.err)
	}
}

func TestMenuEscape(t *testing.T) {
	m := press(t, discordModel(t), keyEsc)
	if m.menu == nil || m.menu.right {
		t.Fatalf("esc on the right: %+v", m.menu)
	}
	if m = press(t, m, keyEsc); m.menu != nil {
		t.Error("esc on the left left the menu open")
	}
	for _, k := range []tea.KeyMsg{keyRunes("q"), altM} {
		if m := press(t, discordModel(t), k); m.menu != nil {
			t.Errorf("%s on the right left the menu open", k)
		}
	}
	if m := press(t, discordModel(t), keyRunes("h")); m.menu == nil || m.menu.right {
		t.Errorf("h on the right: %+v", m.menu)
	}
}
