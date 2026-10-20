// Package herdr öffnet Panes im Terminal-Workspace Herdr, wenn lazydev darin läuft.
package herdr

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Available meldet, ob lazydev in einem Herdr-Pane läuft und das herdr-Programm findet.
func Available() bool {
	if os.Getenv("HERDR_ENV") != "1" {
		return false
	}
	_, err := exec.LookPath(binary())
	return err == nil
}

func binary() string {
	if p := os.Getenv("HERDR_BIN_PATH"); p != "" {
		return p
	}
	return "herdr"
}

// safeArg passt ohne Anführungszeichen in fish, bash und zsh.
var safeArg = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

// Open teilt den aktuellen Pane nach rechts, startet dort eine Shell in dir und führt args aus.
// Ohne args bleibt die Shell leer. Argumente mit Sonderzeichen übergibt Open als
// Umgebungsvariablen, damit das Quoting nicht von der Shell des Menschen abhängt.
func Open(dir string, args []string) (paneID string, err error) {
	split := []string{"pane", "split", "--current", "--direction", "right", "--focus", "--cwd", dir}
	words := make([]string, len(args))
	for i, a := range args {
		if safeArg.MatchString(a) {
			words[i] = a
			continue
		}
		name := "LAZYDEV_ARG" + strconv.Itoa(i)
		split = append(split, "--env", name+"="+a)
		words[i] = `"$` + name + `"`
	}
	out, err := exec.Command(binary(), split...).Output()
	if err != nil {
		return "", fmt.Errorf("herdr pane split: %w", commandError(err))
	}
	var res struct {
		Result struct {
			Pane struct {
				PaneID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &res); err != nil || res.Result.Pane.PaneID == "" {
		return "", fmt.Errorf("herdr pane split: unerwartete Antwort %s", strings.TrimSpace(string(out)))
	}
	paneID = res.Result.Pane.PaneID
	if len(words) == 0 {
		return paneID, nil
	}
	if out, err := exec.Command(binary(), "pane", "run", paneID, strings.Join(words, " ")).CombinedOutput(); err != nil {
		return paneID, fmt.Errorf("herdr pane run: %s", strings.TrimSpace(string(out)))
	}
	return paneID, nil
}

func commandError(err error) error {
	if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) > 0 {
		return fmt.Errorf("%s", strings.TrimSpace(string(exit.Stderr)))
	}
	return err
}
