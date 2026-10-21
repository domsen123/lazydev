package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"lazydev/internal/claude"
	"lazydev/internal/state"
	"lazydev/internal/usage"
)

// logger writes the runner's own lines as JSONL into the log of a step, next to Claude's
// stream-json, which it takes unchanged. Rate limit events of the stream go to package usage.
type logger struct {
	mu   sync.Mutex
	file *os.File
}

// openLog appends to path. With an empty path the logger writes nothing.
func openLog(path string) (*logger, error) {
	l := &logger{}
	if path == "" {
		return l, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	l.file = f
	return l, err
}

func (l *logger) close() {
	if l.file != nil {
		l.file.Close()
	}
}

func (l *logger) printf(format string, args ...any) {
	data, _ := json.Marshal(map[string]string{"type": claude.RunnerType, "time": state.Now(), "text": fmt.Sprintf(format, args...)})
	l.Write(append(data, '\n'))
}

// Write appends a finished JSONL line, e.g. an event of Claude's stream-json.
func (l *logger) Write(line []byte) (int, error) {
	usage.Record(line)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Write(line)
	}
	return len(line), nil
}

// lines returns a writer that logs every line on its own, e.g. the output of a hook.
func (l *logger) lines() io.WriteCloser {
	return &lineWriter{l: l}
}

type lineWriter struct {
	l   *logger
	buf []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.l.printf("%s", bytes.TrimRight(w.buf[:i], "\r"))
		w.buf = w.buf[i+1:]
	}
}

func (w *lineWriter) Close() error {
	if len(w.buf) > 0 {
		w.l.printf("%s", w.buf)
	}
	return nil
}
