// Package git runs the git binary. lazydev uses no Go git library on purpose.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run runs git in dir and returns stdout without the final newline. On failure the error holds
// stderr.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	// lazydev polls status in folders the user works in. Without optional locks status does not
	// refresh the index, so it never makes a git command of the user fail on index.lock. Locks a
	// command needs, like a commit's, stay.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
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

// Lines splits git output into non-empty lines. It keeps leading spaces, porcelain needs them.
func Lines(out string) []string {
	var lines []string
	for l := range strings.SplitSeq(out, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// Output runs git in dir and returns its lines, nil on errors.
func Output(dir string, args ...string) []string {
	out, err := Run(dir, args...)
	if err != nil {
		return nil
	}
	return Lines(out)
}

// IsTracked reports whether path (relative to dir) is in the index.
func IsTracked(dir, path string) bool {
	_, err := Run(dir, "ls-files", "--error-unmatch", "--", path)
	return err == nil
}

// BranchExists reports whether the local branch exists.
func BranchExists(dir, branch string) bool {
	_, err := Run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// Upstream is the upstream of the checked out branch, e.g. origin/main, empty without upstream.
func Upstream(dir string) string {
	u, _ := Run(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	return u
}

// CurrentBranch is the checked out branch, empty for a detached HEAD or outside a repository.
func CurrentBranch(dir string) string {
	b, _ := Run(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	return b
}

// MainDir is the main folder of the repository dir belongs to, also from a subfolder or a linked
// worktree. Outside a repository or in a bare one it is dir.
func MainDir(dir string) string {
	common, err := Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err == nil && filepath.Base(common) == ".git" {
		return filepath.Dir(common)
	}
	if top, err := Run(dir, "rev-parse", "--show-toplevel"); err == nil {
		return top
	}
	return dir
}
