// Package devserver startet und stoppt Dev-Server in Worktrees. Die Server laufen losgelöst von
// der TUI, damit sie einen Neustart von lazydev überstehen und der Merge sie stoppen kann.
package devserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lazydev/internal/state"
)

// PIDPath ist eine eigene Datei, weil der Runner den Lauf-Zustand laufend überschreibt.
func PIDPath(projectID, prdName string) string {
	return filepath.Join(state.RunsDir(projectID), prdName+".devpid")
}

func LogPath(projectID, prdName string) string {
	return filepath.Join(state.RunsDir(projectID), prdName+".dev.log")
}

// PID liefert die PID eines laufenden Dev-Servers, sonst 0. Die Datei enthält PID und Ordner.
// Ein fremder Prozess mit wiederverwendeter PID arbeitet in einem anderen Ordner und zählt nicht.
func PID(projectID, prdName string) int {
	data, err := os.ReadFile(PIDPath(projectID, prdName))
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

// Start führt command in dir losgelöst aus. stdout und stderr landen in LogPath.
func Start(projectID, prdName, dir string, command []string) (int, error) {
	if pid := PID(projectID, prdName); pid != 0 {
		return 0, fmt.Errorf("Dev-Server läuft schon (PID %d)", pid)
	}
	if len(command) == 0 {
		return 0, errors.New("kein Dev-Befehl für dieses Projekt")
	}
	if err := os.MkdirAll(state.RunsDir(projectID), 0o755); err != nil {
		return 0, err
	}
	out, err := os.OpenFile(LogPath(projectID, prdName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Wait räumt den Prozess ab, falls er endet, solange die TUI läuft.
	go cmd.Wait()
	if err := os.WriteFile(PIDPath(projectID, prdName), []byte(strconv.Itoa(pid)+"\n"+dir+"\n"), 0o644); err != nil {
		return pid, err
	}
	return pid, nil
}

// Stop beendet die Prozessgruppe des Dev-Servers: erst SIGTERM, nach 5 Sekunden SIGKILL.
func Stop(projectID, prdName string) error {
	defer os.Remove(PIDPath(projectID, prdName))
	pid := PID(projectID, prdName)
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

// Remove löscht PID-Datei und Log, etwa beim Verwerfen eines Laufs.
func Remove(projectID, prdName string) error {
	var errs []error
	for _, p := range []string{PIDPath(projectID, prdName), LogPath(projectID, prdName)} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
