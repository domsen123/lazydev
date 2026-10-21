package chatrun

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"syscall"
	"time"

	"lazydev/internal/state"
)

// Model gilt für jede Runde. Das Erstellen eines PRDs läuft immer mit Opus, auch wenn die Frontmatter des
// Commands fehlt oder der Nutzer ein anderes Standardmodell hat.
const Model = "opus"

// AllowedTools erlaubt dem Chat das Lesen des Projekts und das Schreiben unter prd/.
// Die Bash-Freigaben decken die Kontextbefehle der Prompts ab, die pencil-Freigaben das Lesen
// von Pencil-Screens.
var AllowedTools = []string{
	"Read", "Glob", "Grep", "Task", "Edit(prd/**)",
	"Bash(find:*)", "Bash(ls:*)", "Bash(git log:*)", "Bash(git status:*)",
	"mcp__pencil__get_app_state", "mcp__pencil__read_skill", "mcp__pencil__execute",
}

// idFormat benennt einen Chat nach seiner Startzeit.
const idFormat = "2006-01-02-150405"

// Create legt den Zustand eines neuen Chats ohne Runde an. Gibt es die Chat-ID der Startzeit
// schon, hängt Create "-2", "-3" usw. an.
func Create(projectID, title, description, dir string, now time.Time) (id string, err error) {
	base := now.Format(idFormat)
	if err := os.MkdirAll(state.ChatsDir(projectID), 0o755); err != nil {
		return "", err
	}
	for n := 1; ; n++ {
		id = base
		if n > 1 {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		// O_EXCL reserviert die ID, auch wenn zwei Chats in derselben Sekunde starten.
		f, err := os.OpenFile(state.ChatPath(projectID, id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		f.Close()
		break
	}
	unlock, err := state.Lock(lockName(projectID, id))
	if err != nil {
		return "", err
	}
	defer unlock()
	ts := now.Format(time.RFC3339)
	c := &Chat{Title: title, Description: description, Dir: dir, Status: StatusDone, StartedAt: ts, UpdatedAt: ts}
	if err := save(projectID, id, c); err != nil {
		os.Remove(state.ChatPath(projectID, id))
		return "", err
	}
	return id, nil
}

// spawnChat startet "lazydev chat" losgelöst in einer eigenen Sitzung und liefert die PID.
// Tests ersetzen die Funktion über SetSpawn.
var spawnChat = func(projectID, id string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	// Ohne Stdout und Stderr schreibt exec nach /dev/null. Die Runde schreibt ihr eigenes Log.
	cmd := exec.Command(exe, "chat", projectID, id)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	return pid, cmd.Process.Release()
}

// SetSpawn ersetzt den Start einer Runde, etwa für Tests in anderen Paketen. restore stellt
// den Start über os.Executable() wieder her.
func SetSpawn(spawn func(projectID, id string) (int, error)) (restore func()) {
	prev := spawnChat
	spawnChat = spawn
	return func() { spawnChat = prev }
}

// Send startet eine Runde mit prompt und zeigt im Verlauf shown an. Send hält die Sperre, bis
// die PID gespeichert ist. Die neue Runde wartet auf dieselbe Sperre.
func Send(projectID, id, shown, prompt string) error {
	unlock, err := state.Lock(lockName(projectID, id))
	if err != nil {
		return err
	}
	defer unlock()
	c, err := Load(projectID, id)
	if err != nil {
		return err
	}
	c.Prompt, c.Shown = prompt, shown
	c.Status, c.Error, c.PID = StatusRunning, "", 0
	pid, err := spawnChat(projectID, id)
	if err != nil {
		c.Status, c.Error = StatusError, err.Error()
		c.UpdatedAt = state.Now()
		if saveErr := save(projectID, id, c); saveErr != nil {
			return errors.Join(err, saveErr)
		}
		return err
	}
	c.PID = pid
	return save(projectID, id, c)
}

// Stop bricht eine laufende Runde mit SIGTERM ab und löscht den Zustand. Das Log bleibt.
func Stop(projectID, id string) error {
	unlock, err := state.Lock(lockName(projectID, id))
	if err != nil {
		return err
	}
	defer unlock()
	c, err := Load(projectID, id)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if c != nil && c.PID > 0 && state.IsChat(c.PID) {
		// Eine losgelöste Runde leitet ihre Sitzung, die PID ist also auch die Gruppen-ID.
		if err := syscall.Kill(-c.PID, syscall.SIGTERM); err != nil {
			if err := syscall.Kill(c.PID, syscall.SIGTERM); err != nil {
				return err
			}
		}
	}
	return Remove(projectID, id)
}
