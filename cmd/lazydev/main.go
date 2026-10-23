// lazydev starts the TUI without arguments. The commands control the run of a PRD.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lazydev/internal/config"
	"lazydev/internal/fsutil"
	"lazydev/internal/git"
	"lazydev/internal/gitcmd"
	"lazydev/internal/project"
	"lazydev/internal/prompts"
	"lazydev/internal/repo"
	"lazydev/internal/runner"
	"lazydev/internal/skill"
	"lazydev/internal/state"
	"lazydev/internal/tui"
	"lazydev/internal/version"
)

const usage = `Usage:
  lazydev                                  start the TUI
  lazydev start <project> <prd>            start or continue the runner of a PRD in the background
  lazydev stop <project> <prd>             stop the runner, the worktree stays
  lazydev discard [--yes] <project> <prd>  throw away the run: worktree, branch and its commits
  lazydev trust <project>                  check and trust hooks and allowed tools of .lazydev/config.yaml
  lazydev update                           install the newest release over this binary
  lazydev prompt create [<description>]    print the create prompt of the project in the current folder
  lazydev skill install                    install the Claude Code skill lazydev into ~/.claude/skills/
  lazydev version                          print the version, also --version and -v

<project> is the project ID (its folder name) or its path, <prd> the PRD file name without .md.
Projects get added in the TUI. After an error or abort, start continues with the failed step.

Projects: %s
State and logs: %s
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		if err := tui.Run(); err != nil {
			fail(err)
		}
		return
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "-h", "--help", "help":
		fmt.Printf(usage, config.Dir(), state.Dir())
		return
	case "version", "--version", "-v":
		fmt.Println("lazydev " + version.Version)
		return
	case "update":
		if len(rest) != 0 {
			usageError()
		}
		if err := update(); err != nil {
			fail(err)
		}
		return
	case "run":
		// "lazydev run <id> <dir> <prd>" is the detached runner the TUI and start spawn.
		if len(rest) != 3 {
			usageError()
		}
		if err := runner.RunMain(rest[0], rest[1], rest[2]); err != nil {
			fail(err)
		}
		return
	case "git":
		// "lazydev git <id> <dir> <action> [<message>]" runs an action of the git cockpit in a terminal tab.
		if len(rest) != 3 && len(rest) != 4 {
			usageError()
		}
		message := ""
		if len(rest) == 4 {
			message = rest[3]
		}
		os.Exit(gitcmd.Main(rest[0], rest[1], rest[2], message))
	case "trust":
		if len(rest) != 1 {
			usageError()
		}
		if err := trust(rest[0]); err != nil {
			fail(err)
		}
		return
	case "prompt":
		// Only the create prompt is printed: a normal Claude Code session writes a PRD with it.
		if len(rest) == 0 {
			usageError()
		}
		if rest[0] != "create" {
			fmt.Fprintf(os.Stderr, "unknown step: %s\n\n", rest[0])
			usageError()
		}
		if err := promptCreate(rest[1:]); err != nil {
			fail(err)
		}
		return
	case "skill":
		if len(rest) != 1 || rest[0] != "install" {
			usageError()
		}
		changed, err := skill.Install()
		if err != nil {
			fail(err)
		}
		if changed {
			fmt.Printf("Installed skill %s to %s.\n", skill.Name, skill.Dir())
		} else {
			fmt.Printf("Skill %s in %s is up to date.\n", skill.Name, skill.Dir())
		}
		return
	case "start", "stop", "discard":
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usageError()
	}

	yes := false
	if cmd == "discard" && len(rest) > 0 && rest[0] == "--yes" {
		yes, rest = true, rest[1:]
	}
	if len(rest) != 2 {
		usageError()
	}
	t, err := runner.Find(rest[0])
	if err != nil {
		fail(err)
	}
	prdName := rest[1]
	switch cmd {
	case "start":
		started, err := runner.Start(t, prdName)
		switch {
		case err != nil:
			fail(err)
		case started:
			fmt.Printf("Runner started. Log: %s\n", state.RunnerLogPath(t.ID, prdName))
		default:
			fmt.Printf("%d runners work already. %s waits and starts once one ends.\n", runner.MaxParallel, prdName)
		}
	case "stop":
		if err := runner.Stop(t.ID, prdName); err != nil {
			fail(err)
		}
		fmt.Println("SIGTERM sent.")
	case "discard":
		if !yes && !confirm(fmt.Sprintf("Delete worktree and branch %s%s of %s? Unmerged commits get lost. (y/n) ", project.FeaturePrefix, prdName, t.ID)) {
			fmt.Println("Cancelled.")
			return
		}
		if err := runner.Discard(t, prdName); err != nil {
			fail(err)
		}
	}
}

// update installs the newest release over the running binary.
func update() error {
	if _, _, _, ok := version.Release(version.Version); !ok {
		return fmt.Errorf("lazydev %s is no release build, update it from source", version.Version)
	}
	ctx := context.Background()
	latest, err := version.Latest(ctx)
	if err != nil {
		return err
	}
	if !version.Newer(latest, version.Version) {
		fmt.Printf("lazydev %s is the newest release.\n", version.Version)
		return nil
	}
	path, err := os.Executable()
	if err == nil {
		path, err = filepath.EvalSymlinks(path)
	}
	if err != nil {
		return err
	}
	if err := version.Install(ctx, latest, path); err != nil {
		return err
	}
	fmt.Printf("Installed lazydev %s to %s. Restart lazydev to use it.\n", latest, path)
	return nil
}

// promptCreate prints the create prompt of the project the working directory belongs to, the same
// prompt n builds in the TUI. args join to $ARGUMENTS, without them the task comes from the
// conversation. The command creates nothing, a project without .lazydev/ is an error.
func promptCreate(args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	if top, err := git.Run(dir, "rev-parse", "--show-toplevel"); err == nil {
		dir = top
	}
	if !fsutil.IsDir(filepath.Join(dir, repo.Dir)) {
		return fmt.Errorf("%s has no %s/, add the project in lazydev first", dir, repo.Dir)
	}
	info := repo.Read(dir, filepath.Base(dir))
	if info.Err != nil {
		return info.Err
	}
	arguments := strings.Join(args, " ")
	if arguments == "" {
		arguments = "Die Aufgabe ergibt sich aus dem bisherigen Gespräch mit dem Nutzer."
	}
	prompt, err := prompts.Build(dir, dir, info.Config, "create", prompts.Vars{Arguments: arguments, Base: info.Config.Base(), Branch: git.CurrentBranch(dir)})
	if err != nil {
		return err
	}
	fmt.Print(prompt)
	return nil
}

// trust shows hooks and allowed tools of .lazydev/config.yaml and saves their hash after "y".
func trust(idOrPath string) error {
	t, err := runner.Find(idOrPath)
	if err != nil {
		return err
	}
	info := repo.Read(t.Dir, t.ID)
	if info.Err != nil {
		return info.Err
	}
	hash := info.Config.TrustHash()
	if hash == "" {
		fmt.Printf("%s sets no hooks or allowed tools in %s/config.yaml, nothing to trust.\n", t.ID, repo.Dir)
		return nil
	}
	fmt.Printf("%s/config.yaml of %s sets hooks or allowed tools:\n\n%s\n", repo.Dir, t.ID, info.Config.TrustSections())
	if !confirm("Trust them? (y/n) ") {
		fmt.Println("Cancelled.")
		return nil
	}
	if err := state.Trust(t.ID, hash); err != nil {
		return err
	}
	fmt.Println("Trusted.")
	return nil
}

func confirm(question string) bool {
	fmt.Print(question)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	a := strings.ToLower(strings.TrimSpace(answer))
	return a == "y" || a == "j"
}

func usageError() {
	fmt.Fprintf(os.Stderr, usage, config.Dir(), state.Dir())
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lazydev:", err)
	os.Exit(1)
}
