package devserver

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"lazydev/internal/state"
)

func TestStartStop(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	pid, err := Start("p", "x", dir, []string{"sleep", "30"}, []string{"LAZYDEV_PORT=3005"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL) })
	if got := PID("p", "x"); got != pid {
		t.Errorf("PID: %d, want %d", got, pid)
	}
	environ, _ := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if !strings.Contains(string(environ), "\x00LAZYDEV_PORT=3005\x00") && !strings.HasPrefix(string(environ), "LAZYDEV_PORT=3005\x00") {
		t.Error("env missing in dev server")
	}
	if cwd, _ := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd"); cwd != dir {
		t.Errorf("cwd: %s", cwd)
	}
	// The server runs in its own session and process group.
	if pgid, _ := syscall.Getpgid(pid); pgid != pid {
		t.Errorf("process group %d, want %d", pgid, pid)
	}
	if _, err := Start("p", "x", dir, []string{"sleep", "30"}, nil); err == nil || !strings.Contains(err.Error(), strconv.Itoa(pid)) {
		t.Errorf("second Start: %v", err)
	}

	if err := Stop("p", "x"); err != nil {
		t.Fatal(err)
	}
	if syscall.Kill(-pid, 0) == nil {
		t.Error("process group still alive")
	}
	if _, err := os.Stat(state.DevPIDPath("p", "x")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("PID file: %v", err)
	}
	if got := PID("p", "x"); got != 0 {
		t.Errorf("PID after Stop: %d", got)
	}
	// Stop without a server is fine.
	if err := Stop("p", "x"); err != nil {
		t.Error(err)
	}
}

func TestStopKillsChildren(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := t.TempDir() + "/child"
	// sh waits for its child, the child writes its PID.
	pid, err := Start("p", "x", t.TempDir(), []string{"sh", "-c", "sleep 30 & echo $! > " + out + "; wait"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL) })
	var child int
	for range 50 {
		data, _ := os.ReadFile(out)
		if child, _ = strconv.Atoi(strings.TrimSpace(string(data))); child > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if child == 0 {
		t.Fatal("child did not start")
	}
	if err := Stop("p", "x"); err != nil {
		t.Fatal(err)
	}
	// The child is no child of this process, so it may linger as a zombie of init for a moment.
	for range 50 {
		if data, err := os.ReadFile("/proc/" + strconv.Itoa(child) + "/stat"); err != nil || strings.Contains(string(data), ") Z ") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("child of the dev server survived Stop")
}

func TestStartErrors(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, err := Start("p", "x", t.TempDir(), nil, nil); err == nil {
		t.Error("no error without command")
	}
	if _, err := Start("p", "x", t.TempDir(), []string{"/nonexistent/dev"}, nil); err == nil {
		t.Error("no error for a missing binary")
	}
	if got := PID("p", "x"); got != 0 {
		t.Errorf("PID after failed Start: %d", got)
	}
}

func TestPIDChecksFolder(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	os.MkdirAll(state.RunsDir("p"), 0o755)
	self := strconv.Itoa(os.Getpid())
	wd, _ := os.Getwd()
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(state.DevPIDPath("p", "x"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A living PID that works in another folder is a reused PID.
	write(self + "\n/somewhere/else\n")
	if got := PID("p", "x"); got != 0 {
		t.Errorf("PID in another folder: %d", got)
	}
	write(self + "\n" + wd + "\n")
	if got := PID("p", "x"); got != os.Getpid() {
		t.Errorf("PID in its folder: %d", got)
	}
	for _, content := range []string{"", "abc\n/x\n", "0\n/x\n", "-5\n/x\n", "999999999\n/x\n"} {
		write(content)
		if got := PID("p", "x"); got != 0 {
			t.Errorf("PID for %q: %d", content, got)
		}
	}
}
