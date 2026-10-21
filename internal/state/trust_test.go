package state

import "testing"

func TestTrust(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if !Trusted("app", "") {
		t.Error("leerer Hash nicht vertraut")
	}
	if Trusted("app", "abc") {
		t.Error("ohne Bestätigung vertraut")
	}
	if err := Trust("app", "abc"); err != nil {
		t.Fatal(err)
	}
	if !Trusted("app", "abc") || Trusted("app", "def") || Trusted("other", "abc") {
		t.Error("Bestätigung gilt für den falschen Hash oder das falsche Projekt")
	}
}
