package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazydev/internal/claude"
	"lazydev/internal/state"
	"lazydev/internal/usage"
)

// limitsStaleAfter markiert einen Stand als alt. Danach zeigt die Anzeige sein Alter.
const limitsStaleAfter = 15 * time.Minute

// pingModel und pingPrompt halten die Anfrage zum Aktualisieren klein.
const (
	pingModel  = "haiku"
	pingPrompt = "Antworte nur mit OK."
)

var windowLabels = map[string]string{
	"five_hour": "5h",
	"seven_day": "7d",
}

var windowTitles = map[string]string{
	"five_hour": "5-Stunden-Fenster",
	"seven_day": "7-Tage-Fenster",
}

type limitsLoadedMsg struct {
	snap usage.Snapshot
	ok   bool
}

type limitsRefreshedMsg struct{ err error }

func loadLimits() tea.Msg {
	s, ok, _ := usage.Load()
	return limitsLoadedMsg{snap: s, ok: ok}
}

// refreshLimits schickt eine kurze Anfrage an Claude. Das Ereignis rate_limit_event darin
// speichert usage.Record.
func (m Model) refreshLimits() tea.Cmd {
	binary := m.cfg.Claude.Binary
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		dir := state.Dir()
		os.MkdirAll(dir, 0o755)
		s, exit, err := claude.Run(ctx, claude.Options{
			Binary: binary, Dir: dir, Prompt: pingPrompt, Model: pingModel, Log: recordWriter{},
		})
		if err == nil && (exit != 0 || s.IsError) {
			err = fmt.Errorf("claude endete mit Exit-Code %d: %s", exit, claude.FirstLine(s.Result))
		}
		return limitsRefreshedMsg{err: err}
	}
}

type recordWriter struct{}

func (recordWriter) Write(line []byte) (int, error) {
	usage.Record(line)
	return len(line), nil
}

// utilizationStyle färbt ab 50 % gelb und ab 80 % rot.
func utilizationStyle(u float64) lipgloss.Style {
	switch {
	case u >= 0.8:
		return styleError
	case u >= 0.5:
		return styleWarn
	}
	return styleOK
}

func percent(u float64) string {
	return fmt.Sprintf("%.0f%%", u*100)
}

// limitsBadge ist die kurze Anzeige unten rechts, z. B. "5h 11% · 7d 37%".
func (m Model) limitsBadge(now time.Time) string {
	if !m.limitsOK {
		return styleMuted.Render("Limits: u")
	}
	var parts []string
	for _, w := range m.limits.Windows {
		label, ok := windowLabels[w.Name]
		if !ok {
			continue
		}
		parts = append(parts, styleMuted.Render(label+" ")+utilizationStyle(w.Utilization).Render(percent(w.Utilization)))
	}
	out := strings.Join(parts, styleMuted.Render(" · "))
	if age := now.Sub(m.limits.Time); age > limitsStaleAfter {
		out += styleMuted.Render(" (vor " + duration(age) + ")")
	}
	if m.limits.Status != "" && m.limits.Status != "allowed" {
		out = styleError.Render(m.limits.Status+" ") + out
	}
	return out + " "
}

// duration schreibt eine Dauer grob: "12 min", "3 h 5 min", "2 T 4 h".
func duration(d time.Duration) string {
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d T %d h", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d h %d min", hours, mins)
	}
	return fmt.Sprintf("%d min", mins)
}

func bar(u float64, width int) string {
	filled := min(width, max(0, int(u*float64(width)+0.5)))
	return utilizationStyle(u).Render(strings.Repeat("█", filled)) + styleMuted.Render(strings.Repeat("░", width-filled))
}

var weekdays = []string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}

// resetTime zeigt die Uhrzeit, bei mehr als einem Tag Abstand mit Wochentag.
func resetTime(t, now time.Time) string {
	t = t.Local()
	if t.Sub(now) > 20*time.Hour {
		return weekdays[t.Weekday()] + " " + t.Format("15:04")
	}
	return t.Format("15:04")
}

// costs summiert die Kosten der Läufe in lazydev, heute und gesamt.
func (m Model) costs(now time.Time) (today, total float64) {
	day := now.Format("2006-01-02")
	for _, r := range m.runs {
		for _, s := range r.Steps {
			if s.CostUSD == nil {
				continue
			}
			total += *s.CostUSD
			if strings.HasPrefix(s.StartedAt, day) {
				today += *s.CostUSD
			}
		}
	}
	return today, total
}

func (m Model) viewLimits(now time.Time) string {
	var lines []string
	if !m.limitsOK {
		lines = append(lines, "Noch kein Stand. lazydev liest die Limits aus jedem Aufruf von claude -p.",
			"Drücke r für eine kurze Anfrage mit Haiku.")
	} else {
		s := m.limits
		lines = append(lines, styleMuted.Render(fmt.Sprintf("Stand %s (vor %s), aus dem letzten Claude-Aufruf von lazydev",
			s.Time.Local().Format("15:04:05"), duration(now.Sub(s.Time)))), "")
		for _, w := range s.Windows {
			title := windowTitles[w.Name]
			if title == "" {
				title = w.Name
			}
			reset := ""
			if !w.ResetsAt.IsZero() {
				reset = fmt.Sprintf("Reset %s (in %s)", resetTime(w.ResetsAt, now), duration(w.ResetsAt.Sub(now)))
			}
			lines = append(lines, fmt.Sprintf("%s %s %s  %s", padRight(title, 18), bar(w.Utilization, 24),
				utilizationStyle(w.Utilization).Render(padLeft(percent(w.Utilization), 4)), styleMuted.Render(reset)))
		}
		lines = append(lines, "")
		status := "Status " + s.Status
		if s.OverageStatus != "" {
			status += styleMuted.Render("  ·  ") + "Extra-Nutzung " + s.OverageStatus
			if s.OverageReason != "" {
				status += " (" + s.OverageReason + ")"
			}
		}
		if s.IsUsingOverage {
			status += styleMuted.Render("  ·  ") + styleError.Render("nutzt Extra-Nutzung")
		}
		lines = append(lines, status)
	}
	today, total := m.costs(now)
	lines = append(lines, "", fmt.Sprintf("Läufe in lazydev: heute %.2f USD, gesamt %.2f USD", today, total),
		styleMuted.Render("Rechnerischer API-Preis aus total_cost_usd, nicht der Abrechnungsbetrag des Abos."))
	if m.limitsRefreshing {
		lines = append(lines, "", styleSelected.Render("● Frage Claude …"))
	}
	lines = append(lines, "", styleKey.Render("r")+" aktualisieren (kurze Anfrage mit Haiku)"+
		styleMuted.Render(" · ")+styleKey.Render("Esc")+" schließen")
	width := min(m.width-4, 96)
	return dialog("Claude-Limits", lines, width, min(m.height-2, len(lines)+4))
}

func (m Model) updateLimits(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		if m.limitsRefreshing {
			return m, nil
		}
		m.limitsRefreshing = true
		return m, m.refreshLimits()
	case "esc", "q", "u":
		m.mode = modeNormal
	}
	return m, nil
}
