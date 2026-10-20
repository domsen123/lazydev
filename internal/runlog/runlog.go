// Package runlog schreibt eigene Ausgaben des Runners als JSONL in das Log eines Schritts.
// Die Zeilen stehen neben dem stream-json von Claude und tragen den Typ "lazydev".
package runlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventType unterscheidet Runner-Zeilen von den Ereignissen aus stream-json.
const EventType = "lazydev"

type Event struct {
	Type string `json:"type"`
	Time string `json:"time"`
	Text string `json:"text"`
}

type Logger struct {
	mu   sync.Mutex
	file *os.File
	echo io.Writer
}

// Open hängt an die Datei unter path an. Mit leerem path schreibt der Logger nur nach echo.
// echo darf nil sein.
func Open(path string, echo io.Writer) (*Logger, error) {
	l := &Logger{echo: echo}
	if path == "" {
		return l, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	l.file = f
	return l, nil
}

func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *Logger) Printf(format string, args ...any) {
	l.write(fmt.Sprintf(format, args...))
}

func (l *Logger) write(text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.file != nil {
		data, _ := json.Marshal(Event{Type: EventType, Time: now.Format(time.RFC3339), Text: text})
		l.file.Write(append(data, '\n'))
	}
	if l.echo != nil {
		fmt.Fprintf(l.echo, "%s lazydev %s\n", now.Format("15:04:05"), text)
	}
}

// Append schreibt eine fertige JSONL-Zeile unverändert in die Datei, etwa ein Ereignis aus
// stream-json. echo bekommt die Zeile nicht.
func (l *Logger) Append(line []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Write(line)
	}
}

// Echo schreibt nur nach echo, z. B. eine kurze Fassung eines Claude-Ereignisses.
func (l *Logger) Echo(text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.echo != nil {
		fmt.Fprintf(l.echo, "%s %s\n", time.Now().Format("15:04:05"), text)
	}
}

// Writer liefert einen Writer, der jede Zeile als eigenes Ereignis schreibt, z. B. für die
// Ausgabe eines Befehls. Close schreibt eine angefangene letzte Zeile.
func (l *Logger) Writer() io.WriteCloser {
	return &lineWriter{l: l}
}

type lineWriter struct {
	l   *Logger
	mu  sync.Mutex
	buf []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.l.write(string(bytes.TrimRight(w.buf[:i], "\r")))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func (w *lineWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.l.write(string(w.buf))
		w.buf = nil
	}
	return nil
}
