package chatrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/state"
	"lazydev/internal/usage"
)

// errGone meldet, dass der Zustand während der Runde verschwand, etwa nach Stop.
var errGone = errors.New("Chat gelöscht")

// update lädt den Zustand unter der Sperre, ändert ihn mit fn und speichert ihn. Fehlt die
// Datei, liefert update errGone und legt sie nicht neu an.
func update(projectID, id string, fn func(*Chat)) (*Chat, error) {
	unlock, err := state.Lock(lockName(projectID, id))
	if err != nil {
		return nil, err
	}
	defer unlock()
	c, err := Load(projectID, id)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errGone
	}
	if err != nil {
		return nil, err
	}
	fn(c)
	return c, save(projectID, id, c)
}

// Round führt die Runde aus, die Send vorbereitet hat. Fehlt der Zustand, endet Round ohne
// Fehler. Bei SIGTERM oder SIGINT bricht die Runde ab.
func Round(cfg *config.Config, projectID, id string) error {
	c, err := update(projectID, id, func(c *Chat) { c.PID = os.Getpid() })
	if errors.Is(err, errGone) {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	var s claude.Summary
	exit := 0
	if err = os.MkdirAll(state.ChatsDir(projectID), 0o755); err == nil {
		var log *os.File
		log, err = os.OpenFile(state.ChatLogPath(projectID, id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			s, exit, err = run(ctx, cfg, projectID, id, c, log)
			log.Close()
		}
	}
	return finish(projectID, id, s, exit, err)
}

// run schreibt die Eingabe ins Log und startet claude.
func run(ctx context.Context, cfg *config.Config, projectID, id string, c *Chat, log io.Writer) (claude.Summary, int, error) {
	writeEvent(log, logEvent{Type: "user_prompt", Text: c.Prompt, Shown: c.Shown})
	session := c.SessionID
	return claude.Run(ctx, claude.Options{
		Binary:       cfg.Claude.Binary,
		Dir:          c.Dir,
		Prompt:       c.Prompt,
		AllowedTools: AllowedTools,
		Resume:       c.SessionID,
		Model:        Model,
		Log:          logWriter{log},
		OnChange: func(s claude.Summary) {
			if s.SessionID == "" || s.SessionID == session {
				return
			}
			session = s.SessionID
			update(projectID, id, func(c *Chat) { c.SessionID = session })
		},
	})
}

// finish speichert das Ergebnis der Runde. Fehlt der Zustand, schreibt finish nichts.
func finish(projectID, id string, s claude.Summary, exit int, runErr error) error {
	unlock, err := state.Lock(lockName(projectID, id))
	if err != nil {
		return err
	}
	defer unlock()
	c, err := Load(projectID, id)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	c.CostUSD += s.CostUSD
	if s.SessionID != "" {
		c.SessionID = s.SessionID
	}
	c.UpdatedAt = state.Now()
	c.PID = 0
	c.Error = roundError(s, exit, runErr)
	switch {
	case c.Error != "":
		c.Status = StatusError
		if f, err := os.OpenFile(state.ChatLogPath(projectID, id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			writeEvent(f, logEvent{Type: "lazydev", Text: c.Error})
			f.Close()
		}
	case IsQuestion(s.Result):
		c.Status = StatusQuestion
	default:
		c.Status = StatusDone
	}
	return save(projectID, id, c)
}

// roundError liefert den Fehler einer Runde oder "", wenn sie fehlerfrei endete.
func roundError(s claude.Summary, exit int, err error) string {
	switch {
	case err != nil:
		return err.Error()
	case exit != 0:
		return fmt.Sprintf("claude endete mit Exit-Code %d", exit)
	case !s.HasResult:
		return "Claude lieferte kein Ergebnis"
	case s.IsError:
		return "Claude meldet einen Fehler: " + claude.FirstLine(s.Result)
	}
	return ""
}

// logEvent ist eine eigene Zeile im Log, etwa die Eingabe des Menschen.
type logEvent struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Shown string `json:"shown,omitempty"`
}

// writeEvent hängt eine eigene Zeile mit Zeitstempel an das Log.
func writeEvent(w io.Writer, e logEvent) {
	data, _ := json.Marshal(e)
	w.Write(claude.WithTimestamp(append(data, '\n')))
}

// logWriter schreibt jede stream-json-Zeile mit Zeitstempel ins Log und übernimmt die Limits.
type logWriter struct{ w io.Writer }

func (l logWriter) Write(line []byte) (int, error) {
	usage.Record(line)
	if _, err := l.w.Write(claude.WithTimestamp(line)); err != nil {
		return 0, err
	}
	return len(line), nil
}
