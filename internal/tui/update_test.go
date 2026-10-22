package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/version"
)

// setVersion sets version.Version for the test.
func setVersion(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
}

// fakeGH sets version.GH to a script that prints out.
func fakeGH(t *testing.T, out string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho "+out+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := version.GH
	version.GH = path
	t.Cleanup(func() { version.GH = old })
}

func TestUpdateMsgSetsNote(t *testing.T) {
	setVersion(t, "v0.1.0")
	next, _ := cockpitModel(t).Update(updateMsg{latest: "v0.2.0"})
	m := next.(model)
	if m.latest != "v0.2.0" {
		t.Errorf("latest: %q", m.latest)
	}
	if want := "lazydev v0.2.0 available, you run v0.1.0. Run install.sh again to update."; m.note != want {
		t.Errorf("note: %q", m.note)
	}
}

func TestUpdateMsgKeepsErr(t *testing.T) {
	setVersion(t, "v0.1.0")
	m := cockpitModel(t)
	m.err = "boom"
	next, _ := m.Update(updateMsg{latest: "v0.2.0"})
	m = next.(model)
	if m.latest != "v0.2.0" || m.note != "" || m.err != "boom" {
		t.Errorf("latest %q, note %q, err %q", m.latest, m.note, m.err)
	}
}

func TestCheckUpdateDev(t *testing.T) {
	setVersion(t, "dev")
	if checkUpdate() != nil {
		t.Error("checkUpdate checks a dev build")
	}
}

func TestCheckUpdateFooter(t *testing.T) {
	setVersion(t, "v0.1.0")
	fakeGH(t, "v0.2.0")
	cmd := checkUpdate()
	if cmd == nil {
		t.Fatal("checkUpdate returned nil")
	}
	msg := cmd()
	if _, ok := msg.(updateMsg); !ok {
		t.Fatalf("msg: %#v", msg)
	}
	next, _ := cockpitModel(t).Update(msg)
	plain := ansi.Strip(next.(model).View())
	if !strings.Contains(plain, "lazydev v0.2.0 available, you run v0.1.0. Run install.sh again to update.") {
		t.Errorf("footer:\n%s", plain)
	}
}

func TestCheckUpdateNotNewer(t *testing.T) {
	setVersion(t, "v0.2.0")
	fakeGH(t, "v0.2.0")
	if msg := checkUpdate()(); msg != nil {
		t.Errorf("msg: %#v", msg)
	}
}
