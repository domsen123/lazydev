package chatrun

import "testing"

func TestIsQuestion(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"Welche Variante?\n\n- A: Upload im Profil\n- B: Upload beim Anmelden", true},
		{"**B:** Upload beim Anmelden", true},
		{"1. Frage\n   C) beides", true},
		{"> **D)** keins", true},
		{"Wie groß darf das Bild sein?", true},
		{"Wie groß darf das Bild sein?**\n\n", true},
		{"Soll `x` bleiben?`  ", true},
		{"PRD gespeichert unter prd/x.md.", false},
		{"Ein Absatz mit ? mitten im Satz.", false},
		{"Annahme: Bilder bis 2 MB", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsQuestion(tt.text); got != tt.want {
			t.Errorf("IsQuestion(%q) = %v, erwartet %v", tt.text, got, tt.want)
		}
	}
}
