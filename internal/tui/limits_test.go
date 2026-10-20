package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/usage"
)

func TestLimitsBadgeAndView(t *testing.T) {
	m := setupRun(t)
	now := time.Date(2026, 10, 20, 17, 0, 0, 0, time.Local)
	if got := ansi.Strip(m.limitsBadge(now)); got != "Limits: u" {
		t.Errorf("ohne Stand: %q", got)
	}
	m.limits = usage.Snapshot{Time: now.Add(-20 * time.Minute), Status: "allowed", OverageStatus: "rejected",
		Windows: []usage.Window{
			{Name: "five_hour", Utilization: 0.11, ResetsAt: now.Add(2 * time.Hour)},
			{Name: "seven_day", Utilization: 0.85, ResetsAt: now.Add(72 * time.Hour)},
		}}
	m.limitsOK = true
	if got := ansi.Strip(m.limitsBadge(now)); got != "5h 11% · 7d 85% (vor 20 min) " {
		t.Errorf("Anzeige: %q", got)
	}
	view := ansi.Strip(m.viewLimits(now))
	for _, want := range []string{"5-Stunden-Fenster", "11%", "Reset 19:00 (in 2 h 0 min)", "7-Tage-Fenster", "85%",
		"(in 3 T 0 h)", "Extra-Nutzung rejected", "heute 0.25 USD, gesamt 0.25 USD"} {
		if !strings.Contains(view, want) {
			t.Errorf("Übersicht ohne %q:\n%s", want, view)
		}
	}
	m = drive(t, m, keys("u")...)
	if m.mode != modeLimits {
		t.Errorf("u öffnet die Übersicht nicht")
	}
}
