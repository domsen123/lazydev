// Package devserver starts and stops the dev server of a worktree. Servers run detached from the
// TUI, so they survive a restart of lazydev and the merge can stop them.
package devserver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lazydev/internal/state"
)

// PID is the PID of the running dev server of a PRD, 0 without one. The PID file holds PID and
// folder, a reused PID of another process works in another folder and does not count.
func PID(id, prdName string) int {
	data, err := os.ReadFile(state.DevPIDPath(id, prdName))
	if err != nil {
		return 0
	}
	pidText, dir, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 || syscall.Kill(pid, 0) != nil {
		return 0
	}
	if cwd, err := os.Readlink("/proc/" + pidText + "/cwd"); err == nil && dir != "" && cwd != dir {
		return 0
	}
	return pid
}

// Start runs command in dir detached, with env added to the environment. stdout and stderr go to
// the dev log of the PRD.
func Start(id, prdName, dir string, command, env []string) (int, error) {
	if pid := PID(id, prdName); pid != 0 {
		return 0, fmt.Errorf("dev server runs already (PID %d)", pid)
	}
	if len(command) == 0 {
		return 0, errors.New("no dev command, set hooks.dev in .lazydev/config.yaml")
	}
	if err := os.MkdirAll(state.RunsDir(id), 0o755); err != nil {
		return 0, err
	}
	out, err := os.OpenFile(state.DevLogPath(id, prdName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Wait reaps the process if it ends while the TUI runs.
	go cmd.Wait()
	return pid, os.WriteFile(state.DevPIDPath(id, prdName), []byte(strconv.Itoa(pid)+"\n"+dir+"\n"), 0o644)
}

// Stop ends the process group of the dev server: SIGTERM first, SIGKILL after 5 seconds.
func Stop(id, prdName string) error {
	defer os.Remove(state.DevPIDPath(id, prdName))
	pid := PID(id, prdName)
	if pid == 0 {
		return nil
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	for range 50 {
		if syscall.Kill(-pid, 0) != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
