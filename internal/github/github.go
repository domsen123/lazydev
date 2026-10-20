// Package github lädt die offenen Issues eines Projekts über das CLI gh. lazydev nutzt bewusst
// keine REST-API und keinen eigenen Token.
package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"lazydev/internal/git"
)

// Binary ist das Programm gh. Tests ersetzen den Wert durch ein Fake-Skript.
var Binary = "gh"

// remotePrefixes sind die erkannten Anfänge einer URL auf github.com.
var remotePrefixes = []string{"https://github.com/", "git@github.com:", "ssh://git@github.com/"}

// ParseRemote liefert <owner>/<repo> für eine URL auf github.com. .git und ein abschließender /
// entfallen.
func ParseRemote(url string) (repo string, ok bool) {
	for _, prefix := range remotePrefixes {
		rest, found := strings.CutPrefix(url, prefix)
		if !found {
			continue
		}
		rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")
		owner, name, found := strings.Cut(rest, "/")
		if !found || owner == "" || name == "" || strings.Contains(name, "/") {
			return "", false
		}
		return owner + "/" + name, true
	}
	return "", false
}

// Repo liefert <owner>/<repo> des Remotes origin in dir. Ohne origin oder bei einem anderen Host
// ist ok false.
func Repo(dir string) (string, bool) {
	out, err := git.Run(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", false
	}
	return ParseRemote(strings.TrimSpace(out))
}

// Issue ist ein offenes Issue, wie gh issue list es liefert.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

// List lädt bis zu 30 offene Issues von repo, die neuesten zuerst.
func List(dir, repo string) ([]Issue, error) {
	out, err := run(dir, "issue", "list", "--repo", repo, "--state", "open", "--limit", "30",
		"--json", "number,title,body,url,createdAt")
	if err != nil {
		return nil, err
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("Ausgabe von gh nicht lesbar: %w", err)
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].CreatedAt.After(issues[j].CreatedAt) })
	return issues, nil
}

// OpenWeb öffnet das Issue number im Browser.
func OpenWeb(dir, repo string, number int) error {
	_, err := run(dir, "issue", "view", strconv.Itoa(number), "--repo", repo, "--web")
	return err
}

// run führt gh in dir aus. Bei Exit-Code ungleich 0 enthält der Fehler die Ausgabe von stderr.
func run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(Binary, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if msg := strings.TrimSpace(stderr.String()); errors.As(err, &exit) && msg != "" {
			return nil, errors.New(msg)
		}
		return nil, err
	}
	return out, nil
}
