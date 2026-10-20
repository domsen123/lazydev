package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"lazydev/internal/project"
	"lazydev/internal/state"
)

// watchDebounce sammelt Ereignisse, damit ein Schwall von Schreibzugriffen nur ein Neueinlesen
// auslöst.
const watchDebounce = 150 * time.Millisecond

// refreshInterval prüft regelmäßig, ob Runner noch leben. Ein beendeter Prozess löst kein
// Dateiereignis aus.
const refreshInterval = 2 * time.Second

// watchMsg meldet, was sich auf der Platte geändert hat.
type watchMsg struct {
	prds  bool
	runs  bool
	logs  bool
	chats bool
}

type tickMsg struct{}

type watcher struct {
	w      *fsnotify.Watcher
	events chan watchMsg
}

// newWatcher überwacht prd/ jedes Projekts sowie runs/ und chats/ im Zustandsverzeichnis. Fehlt
// fsnotify, liefert newWatcher nil, und die TUI arbeitet mit dem regelmäßigen Neueinlesen.
func newWatcher(projectDirs []string) *watcher {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil
	}
	w := &watcher{w: fw, events: make(chan watchMsg)}
	for _, dir := range projectDirs {
		w.addTree(filepath.Join(dir, project.PRDDir))
	}
	runsDir := filepath.Join(state.Dir(), "runs")
	chatsDir := filepath.Join(state.Dir(), "chats")
	for _, dir := range []string{runsDir, chatsDir} {
		os.MkdirAll(dir, 0o755)
		w.addTree(dir)
	}
	go w.loop(runsDir, chatsDir)
	return w
}

func (w *watcher) addTree(root string) {
	filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err == nil && e.IsDir() {
			w.w.Add(path)
		}
		return nil
	})
}

// addProject überwacht ein neu eingetragenes Projekt.
func (w *watcher) addProject(dir string) {
	w.addTree(filepath.Join(dir, project.PRDDir))
}

func (w *watcher) Close() {
	w.w.Close()
}

func (w *watcher) loop(runsDir, chatsDir string) {
	var pending watchMsg
	var timer <-chan time.Time
	for {
		select {
		case ev, ok := <-w.w.Events:
			if !ok {
				return
			}
			if ev.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					w.addTree(ev.Name)
				}
			}
			switch {
			case strings.HasPrefix(ev.Name, runsDir):
				if strings.HasSuffix(ev.Name, ".jsonl") || strings.HasSuffix(ev.Name, ".log") {
					pending.logs = true
				} else {
					pending.runs = true
				}
			case strings.HasPrefix(ev.Name, chatsDir):
				if strings.HasSuffix(ev.Name, ".jsonl") {
					pending.logs = true
				} else if strings.HasSuffix(ev.Name, ".json") {
					pending.chats = true
				}
			default:
				pending.prds = true
			}
			if timer == nil {
				timer = time.After(watchDebounce)
			}
		case _, ok := <-w.w.Errors:
			if !ok {
				return
			}
		case <-timer:
			w.events <- pending
			pending = watchMsg{}
			timer = nil
		}
	}
}

// next wartet auf das nächste gesammelte Ereignis.
func (w *watcher) next() tea.Cmd {
	if w == nil {
		return nil
	}
	return func() tea.Msg { return <-w.events }
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}
