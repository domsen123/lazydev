package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lazydev/internal/config"
)

// completePath vervollständigt einen Ordnerpfad wie eine Shell. Gibt es genau einen
// Treffer, hängt completePath einen "/" an. Bei mehreren Treffern ergänzt es bis zum
// gemeinsamen Präfix und liefert die Treffer als Kandidaten zurück.
// Versteckte Ordner erscheinen nur, wenn die Eingabe mit "." beginnt.
func completePath(input string) (completed string, candidates []string) {
	dirPart, prefix := splitPathInput(input)
	entries, err := os.ReadDir(expandDir(dirPart))
	if err != nil {
		return input, nil
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if isDir(filepath.Join(expandDir(dirPart), name), e) {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	switch len(candidates) {
	case 0:
		return input, nil
	case 1:
		return dirPart + candidates[0] + "/", nil
	default:
		return dirPart + commonPrefix(candidates), candidates
	}
}

// splitPathInput trennt "~/dev/nu" in "~/dev/" und "nu".
func splitPathInput(input string) (dir, prefix string) {
	if input == "~" {
		return "~/", ""
	}
	i := strings.LastIndex(input, "/")
	if i < 0 {
		return "", input
	}
	return input[:i+1], input[i+1:]
}

func expandDir(dir string) string {
	if dir == "" {
		return "."
	}
	return config.ExpandPath(dir)
}

// isDir folgt symbolischen Links, damit verlinkte Projektordner vervollständigt werden.
func isDir(path string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// commonPrefix arbeitet auf Runen, damit Umlaute nicht zerschnitten werden.
func commonPrefix(items []string) string {
	p := []rune(items[0])
	for _, s := range items[1:] {
		for !strings.HasPrefix(s, string(p)) {
			p = p[:len(p)-1]
		}
	}
	return string(p)
}
