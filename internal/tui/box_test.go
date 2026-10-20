package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestBoxDropsSoftHyphen prüft, dass ein weicher Trennstrich die Zeile nicht breiter macht.
func TestBoxDropsSoftHyphen(t *testing.T) {
	out := box("Details", []string{"Bestell­verwaltung\tmit\r Tab"}, 30, 3, false)
	for _, l := range strings.Split(out, "\n") {
		if strings.ContainsAny(l, "­\t\r") {
			t.Errorf("Steuerzeichen in %q", l)
		}
	}
	if !strings.Contains(out, "Bestellverwaltung mit Tab") {
		t.Errorf("Text fehlt: %q", out)
	}
}

// TestDialogPadding prüft den Innenabstand von 2 Spalten und 1 Zeile.
func TestDialogPadding(t *testing.T) {
	lines := strings.Split(dialog("T", []string{"x"}, 20, 6), "\n")
	if len(lines) != 6 {
		t.Fatalf("%d Zeilen statt 6:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 20 {
			t.Errorf("Zeile %d hat %d Zellen statt 20: %q", i+1, w, l)
		}
	}
	empty := "│" + strings.Repeat(" ", 18) + "│"
	if got := ansi.Strip(lines[1]); got != empty {
		t.Errorf("Zeile 2 nicht leer: %q", got)
	}
	if got := ansi.Strip(lines[2]); !strings.HasPrefix(got, "│  x") || !strings.HasSuffix(got, "  │") {
		t.Errorf("Zeile 3 ohne Abstand: %q", got)
	}
	if got := ansi.Strip(lines[4]); got != empty {
		t.Errorf("Zeile 5 nicht leer: %q", got)
	}

	long := strings.Split(dialog("T", []string{strings.Repeat("a", 30)}, 20, 5), "\n")
	if got := ansi.Strip(long[2]); !strings.HasSuffix(got, "…  │") || ansi.StringWidth(got) != 20 {
		t.Errorf("lange Zeile nicht gekürzt: %q", got)
	}
}

// TestOverlayDimsBackground prüft, dass overlay die Farben des Hintergrunds entfernt.
func TestOverlayDimsBackground(t *testing.T) {
	row := "\x1b[31mrot\x1b[0m" + strings.Repeat(" ", 37)
	out := overlay(row+"\n"+row, "X", 40)
	if strings.Contains(out, "\x1b[31m") {
		t.Errorf("Hintergrund nicht gedimmt: %q", out)
	}
	if plain := ansi.Strip(out); !strings.Contains(plain, "rot") || !strings.Contains(plain, "X") {
		t.Errorf("Text fehlt: %q", plain)
	}
}
