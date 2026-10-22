package tui

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/version"
)

var altU = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u"), Alt: true}

// setVersion sets version.Version for the test.
func setVersion(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
}

// fakeGitHub serves release tag with a lazydev that contains "new binary".
func fakeGitHub(t *testing.T, tag string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	bin := []byte("new binary")
	tw.WriteHeader(&tar.Header{Name: "lazydev", Mode: 0o755, Size: int64(len(bin)), Typeflag: tar.TypeReg})
	tw.Write(bin)
	tw.Close()
	gz.Close()
	archive := buf.Bytes()
	name := "lazydev_" + strings.TrimPrefix(tag, "v") + "_linux_" + runtime.GOARCH + ".tar.gz"
	sum := sha256.Sum256(archive)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/domsen123/lazydev/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q}`, tag)
	})
	mux.HandleFunc("/domsen123/lazydev/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	})
	mux.HandleFunc("/domsen123/lazydev/releases/download/"+tag+"/"+name, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	oldAPI, oldDownload := version.API, version.Download
	version.API, version.Download = srv.URL, srv.URL
	t.Cleanup(func() { version.API, version.Download = oldAPI, oldDownload })
}

// fakeExecutable points executable at a temporary file with "old binary" and returns its path.
func fakeExecutable(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lazydev")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := executable
	executable = func() (string, error) { return path, nil }
	t.Cleanup(func() { executable = old })
	return path
}

func TestUpdateMsgSetsNote(t *testing.T) {
	setVersion(t, "v0.1.0")
	next, _ := cockpitModel(t).Update(updateMsg{latest: "v0.2.0"})
	m := next.(model)
	if m.latest != "v0.2.0" {
		t.Errorf("latest: %q", m.latest)
	}
	if want := "lazydev v0.2.0 available, you run v0.1.0. alt+u updates."; m.note != want {
		t.Errorf("note: %q", m.note)
	}
	if !m.updateNoteShown() {
		t.Error("update note not marked")
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
	fakeGitHub(t, "v0.2.0")
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
	if !strings.Contains(plain, "lazydev v0.2.0 available, you run v0.1.0. alt+u updates.") {
		t.Errorf("footer:\n%s", plain)
	}
}

func TestCheckUpdateNotNewer(t *testing.T) {
	setVersion(t, "v0.2.0")
	fakeGitHub(t, "v0.2.0")
	if msg := checkUpdate()(); msg != nil {
		t.Errorf("msg: %#v", msg)
	}
}

func TestUpdateKeyHelp(t *testing.T) {
	m := cockpitModel(t)
	m.latest = "v0.2.0"
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, " alt+u update v0.2.0 · j/k select") {
		t.Errorf("footer:\n%s", plain)
	}
}

func TestUpdateKeyInstalls(t *testing.T) {
	setVersion(t, "v0.1.0")
	fakeGitHub(t, "v0.2.0")
	path := fakeExecutable(t)
	m := cockpitModel(t)
	m.latest = "v0.2.0"
	next, cmd := m.Update(altU)
	m = next.(model)
	if !m.updating || m.note != "Updating lazydev to v0.2.0 …" || cmd == nil {
		t.Fatalf("updating %v, note %q, cmd %v", m.updating, m.note, cmd != nil)
	}
	next, _ = m.Update(cmd())
	m = next.(model)
	if m.updating || m.latest != "" || m.installed != "v0.2.0" || m.err != "" {
		t.Errorf("updating %v, latest %q, installed %q, err %q", m.updating, m.latest, m.installed, m.err)
	}
	if want := "lazydev v0.2.0 installed. Restart lazydev to use it."; m.note != want {
		t.Errorf("note: %q", m.note)
	}
	if data, _ := os.ReadFile(path); string(data) != "new binary" {
		t.Errorf("binary %q", data)
	}
	if want := "lazydev v0.1.0 · v0.2.0 installed, restart lazydev."; m.versionLine() != want {
		t.Errorf("version line: %q", m.versionLine())
	}
}

func TestUpdateKeyPlain(t *testing.T) {
	m := cockpitModel(t)
	m.latest = "v0.2.0"
	fakeExecutable(t)
	fakeGitHub(t, "v0.2.0")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if !m.updating {
		t.Errorf("u started no update, note %q", m.note)
	}
}

func TestUpdateKeyWithoutRelease(t *testing.T) {
	m := press(t, cockpitModel(t), altU)
	if m.updating || m.note != "No newer release of lazydev known." {
		t.Errorf("updating %v, note %q", m.updating, m.note)
	}
}

func TestUpdateFails(t *testing.T) {
	m := cockpitModel(t)
	m.latest, m.updating = "v0.2.0", true
	next, _ := m.Update(installMsg{tag: "v0.2.0", err: fmt.Errorf("GET x: 404 Not Found")})
	m = next.(model)
	if m.updating || m.latest != "v0.2.0" || m.err != "update: GET x: 404 Not Found" {
		t.Errorf("updating %v, latest %q, err %q", m.updating, m.latest, m.err)
	}
}
