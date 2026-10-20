// Package git ruft das git-Binärprogramm auf. lazydev nutzt bewusst keine Go-Git-Bibliothek.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run führt git in dir aus und liefert stdout ohne abschließenden Zeilenumbruch.
// Im Fehlerfall enthält der Fehler die erste Zeile von stderr.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// IsTracked meldet, ob path (relativ zu dir) im Index liegt.
func IsTracked(dir, path string) bool {
	_, err := Run(dir, "ls-files", "--error-unmatch", "--", path)
	return err == nil
}

// BranchExists meldet, ob der lokale Branch existiert.
func BranchExists(dir, branch string) bool {
	_, err := Run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// Lines teilt eine git-Ausgabe in nicht leere Zeilen.
func Lines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
