// Package gitcmd runs the actions of the git cockpit in a terminal tab.
package gitcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"lazydev/internal/git"
	"lazydev/internal/state"
)

// The actions of the git cockpit.
const (
	Fetch  = "fetch"
	Pull   = "pull"
	Push   = "push"
	Commit = "commit"
)

// Run runs action in the main folder dir of project id and returns the exit code. Git reads from in
// and writes to out, so prompts for passphrases or credentials work like in a shell. Pull and push
// hold the merge lock of the project, so they never run during the fast-forward of a runner.
func Run(id, dir, action, message string, in io.Reader, out io.Writer) int {
	switch action {
	case Fetch:
		return run(dir, in, out, "fetch")
	case Pull, Push:
		branch := git.CurrentBranch(dir)
		if branch == "" {
			fmt.Fprintln(out, "HEAD is detached, check out a branch first.")
			return 1
		}
		unlock, err := state.TryLock(state.MergeLock(id))
		if err != nil {
			fmt.Fprintln(out, err)
			return 1
		}
		if unlock == nil {
			fmt.Fprintf(out, "A PRD of %s merges right now. Try again once the merge is done.\n", id)
			return 1
		}
		defer unlock()
		if action == Pull {
			return run(dir, in, out, "pull", "--ff-only")
		}
		if git.Upstream(dir) == "" {
			return run(dir, in, out, "push", "-u", "origin", branch)
		}
		return run(dir, in, out, "push")
	case Commit:
		if strings.TrimSpace(message) == "" {
			fmt.Fprintln(out, "Enter a commit message.")
			return 1
		}
		if code := run(dir, in, out, "add", "-A"); code != 0 {
			return code
		}
		return run(dir, in, out, "commit", "-m", message)
	}
	fmt.Fprintf(out, "unknown git action: %s\n", action)
	return 1
}

// run runs git with args in dir after writing the command line to out.
func run(dir string, in io.Reader, out io.Writer, args ...string) int {
	fmt.Fprintf(out, "$ git %s\n", strings.Join(args, " "))
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintln(out, err)
		return 1
	}
	return 0
}

// Main runs action in the terminal of the tab. A tab closes once its program ends, so after a
// failure Main waits for enter to keep the output readable.
func Main(id, dir, action, message string) int {
	code := Run(id, dir, action, message, os.Stdin, os.Stdout)
	if code != 0 {
		fmt.Println()
		fmt.Printf("failed (exit %d), press enter to close\n", code)
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	return code
}
