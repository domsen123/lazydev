package devserver

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartStop(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	pid, err := Start("p", "x", dir, []string{"sh", "-c", "echo bereit; sleep 30"})
	if err != nil || pid == 0 {
		t.Fatalf("Start = %d, %v", pid, err)
	}
	if PID("p", "x") != pid {
		t.Error("PID nicht gespeichert")
	}
	if _, err := Start("p", "x", dir, []string{"true"}); err == nil {
		t.Error("zweiter Start akzeptiert")
	}
	time.Sleep(100 * time.Millisecond)
	if log, _ := os.ReadFile(LogPath("p", "x")); !strings.Contains(string(log), "bereit") {
		t.Errorf("Log: %q", log)
	}
	if err := Stop("p", "x"); err != nil {
		t.Fatal(err)
	}
	if PID("p", "x") != 0 {
		t.Error("Dev-Server läuft nach Stop")
	}
}
