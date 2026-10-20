package herdr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHerdr protokolliert Aufrufe und antwortet wie herdr pane split.
func fakeHerdr(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf '%s|' \"$@\" >> " + logPath + "\necho >> " + logPath + "\n" +
		"[ \"$2\" = split ] && echo '{\"result\":{\"pane\":{\"pane_id\":\"w:p9\"}}}'\nexit 0\n"
	bin := filepath.Join(dir, "herdr")
	os.WriteFile(bin, []byte(script), 0o755)
	t.Setenv("HERDR_BIN_PATH", bin)
	t.Setenv("HERDR_ENV", "1")
	return logPath
}

func TestOpenQuotesUnsafeArgs(t *testing.T) {
	logPath := fakeHerdr(t)
	if !Available() {
		t.Fatal("Herdr nicht erkannt")
	}
	id, err := Open("/tmp/x", []string{"claude", "/create-prd Avatar's Upload"})
	if err != nil || id != "w:p9" {
		t.Fatalf("Open = %q, %v", id, err)
	}
	calls, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Aufrufe:\n%s", calls)
	}
	if !strings.Contains(lines[0], "--cwd|/tmp/x|--env|LAZYDEV_ARG1=/create-prd Avatar's Upload|") {
		t.Errorf("split: %s", lines[0])
	}
	if lines[1] != `pane|run|w:p9|claude "$LAZYDEV_ARG1"|` {
		t.Errorf("run: %s", lines[1])
	}
}

func TestOpenShellOnly(t *testing.T) {
	logPath := fakeHerdr(t)
	if _, err := Open("/tmp/x", nil); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(logPath)
	if strings.Count(string(calls), "\n") != 1 {
		t.Errorf("ohne Befehl kein pane run erwartet:\n%s", calls)
	}
}
