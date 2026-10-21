package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DefaultAllowedTools gilt für jeden Schritt. Die Projekt-Konfiguration ergänzt die Liste.
// Claude ignoriert permissions.allow in Ordnern ohne Trust-Dialog, deshalb kommen alle
// Freigaben über --allowedTools.
var DefaultAllowedTools = []string{
	"Read", "Edit", "Write", "Glob", "Grep", "Task",
	"Bash(git status:*)", "Bash(git diff:*)", "Bash(git log:*)", "Bash(git add:*)",
	"Bash(git commit:*)", "Bash(git switch:*)", "Bash(git rebase:*)", "Bash(git checkout:*)",
	"Bash(git rm:*)", "Bash(git mv:*)", "Bash(git show:*)", "Bash(git branch:*)",
	"Bash(pnpm lint:*)", "Bash(pnpm typecheck:*)", "Bash(pnpm test:run:*)",
	"Bash(pnpm db:generate:*)", "Bash(pnpm db:migrate:*)",
	"mcp__pencil__get_app_state", "mcp__pencil__read_skill", "mcp__pencil__execute",
}

// StderrType markiert Zeilen von stderr im Log, damit das Log durchgehend JSONL bleibt.
const StderrType = "stderr"

// Options beschreibt einen Aufruf von claude -p.
type Options struct {
	Binary       string
	Dir          string
	Prompt       string
	AllowedTools []string
	// Resume setzt eine bestehende Sitzung fort.
	Resume string
	// Model wählt das Modell, z. B. "opus". Leer heißt: Standard des Nutzers oder des Commands.
	Model string
	// Log bekommt jede stdout-Zeile unverändert und jede stderr-Zeile als Ereignis vom Typ
	// "stderr".
	Log io.Writer
	// OnChange läuft, sobald SessionID oder Ergebnis feststehen, etwa um den Zustand zu sichern.
	OnChange func(Summary)
}

// Args liefert die Argumente für claude ohne das Binärprogramm.
func (o Options) Args() []string {
	args := []string{
		"-p", o.Prompt,
		"--output-format", "stream-json", "--verbose",
		"--permission-mode", "acceptEdits",
		"--allowedTools", strings.Join(o.AllowedTools, ","),
		"--agents", AgentsPath(),
	}
	if o.Resume != "" {
		args = append(args, "--resume", o.Resume)
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	return args
}

// Run startet claude und wartet auf das Ende. exitCode ist -1, wenn claude nicht startet oder
// durch ein Signal endet. Bricht ctx ab, bekommt claude SIGTERM.
func Run(ctx context.Context, o Options) (s Summary, exitCode int, err error) {
	if err := WriteAgents(); err != nil {
		return s, -1, err
	}
	log := &syncWriter{w: o.Log}
	cmd := exec.CommandContext(ctx, o.Binary, o.Args()...)
	cmd.Dir = o.Dir
	// Ohne GIT_EDITOR öffnet git rebase --continue einen Editor und wartet.
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 10 * time.Second
	// Ohne stdin wartet claude -p drei Sekunden auf Eingabe.
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return s, -1, err
	}
	defer devnull.Close()
	cmd.Stdin = devnull
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return s, -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return s, -1, err
	}
	if err := cmd.Start(); err != nil {
		return s, -1, err
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			data, _ := json.Marshal(map[string]string{"type": StderrType, "text": sc.Text()})
			log.Write(append(data, '\n'))
		}
	}()

	r := bufio.NewReader(stdout)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				line = append(line, '\n')
			}
			log.Write(line)
			if e, ok := ParseEvent(line); ok && s.Add(e) && o.OnChange != nil {
				o.OnChange(s)
			}
		}
		if readErr != nil {
			break
		}
	}
	<-done

	err = cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return s, 0, nil
	case errors.As(err, &exit):
		return s, exit.ExitCode(), nil
	default:
		return s, -1, err
	}
}

// syncWriter hält die Zeilen von stdout und stderr im Log getrennt.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
