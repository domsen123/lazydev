package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lazydev/internal/git"
)

// commitWords sind die englischen Statuswerte in der Commit-Nachricht.
var commitWords = map[StatusKind]string{
	StatusPlanned:     "planned",
	StatusImplemented: "done",
	StatusDiscarded:   "discarded",
}

// ChangeStatus setzt den Statuswert des PRDs rel (relativ zu projectDir) auf value. Ein getracktes
// PRD committet ChangeStatus im Hauptordner, nur diese Datei und ohne Trailer. Dafür muss der
// Hauptordner auf base stehen und die Datei darf keine uncommitteten Änderungen haben, sonst
// ändert ChangeStatus nichts. Ein ungetracktes PRD ändert ChangeStatus ohne Commit.
func ChangeStatus(projectDir, base, rel, value string) (committed bool, err error) {
	tracked := git.IsTracked(projectDir, rel)
	if tracked {
		branch, err := currentBranch(projectDir)
		if err != nil {
			return false, err
		}
		if branch != base {
			return false, fmt.Errorf("Hauptordner steht auf %s statt %s", branch, base)
		}
		out, err := git.Run(projectDir, "status", "--porcelain", "--", rel)
		if err != nil {
			return false, err
		}
		if out != "" {
			return false, fmt.Errorf("%s hat uncommittete Änderungen", rel)
		}
	}
	path := filepath.Join(projectDir, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content, ok := SetStatus(string(data), value)
	if !ok {
		return false, fmt.Errorf("%s hat keine Statuszeile", rel)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	if !tracked {
		return false, nil
	}
	s, _ := ParseStatusLine("> Status: " + value)
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	msg := fmt.Sprintf("docs(prd): mark %s as %s", name, commitWords[s.Kind])
	if _, err := git.Run(projectDir, "commit", "-m", msg, "--", rel); err != nil {
		return false, err
	}
	return true, nil
}
