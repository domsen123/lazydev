package runner

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"lazydev/internal/devserver"
	"lazydev/internal/fsutil"
	"lazydev/internal/git"
	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

const (
	// PortBase is the first port for worktrees.
	PortBase        = 3500
	portSearchRange = 1000
	// maxSlugLen keeps database names like "<name>_<slug>" below the Postgres limit of 63 bytes.
	maxSlugLen = 40
)

// WorktreePath honors XDG_DATA_HOME.
func WorktreePath(id, prdName string) string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "lazydev", "worktrees", id, prdName)
}

// Slug turns "mobile-bearer-token-auth" into the database suffix "mobile_bearer_token_auth".
func Slug(prdName string) string {
	var b strings.Builder
	underscore := false
	for _, r := range strings.ToLower(prdName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			underscore = false
		} else if !underscore && b.Len() > 0 {
			b.WriteByte('_')
			underscore = true
		}
	}
	s := b.String()
	if len(s) > maxSlugLen {
		s = s[:maxSlugLen]
	}
	return strings.Trim(s, "_")
}

// allocatePort returns the first port from PortBase that no other unmerged run holds and no
// process listens on, on IPv4 and IPv6.
func allocatePort() (int, error) {
	used := map[int]bool{}
	for _, e := range state.All() {
		if e.Run.Phase != project.PhaseMerged {
			used[e.Run.Port] = true
		}
	}
	for p := PortBase; p < PortBase+portSearchRange; p++ {
		if !used[p] && portFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port between %d and %d", PortBase, PortBase+portSearchRange-1)
}

func portFree(port int) bool {
	for _, host := range []string{"127.0.0.1", "::1"} {
		l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			// Without IPv6 nobody can listen there either.
			if host == "::1" && (strings.Contains(err.Error(), "cannot assign requested address") || strings.Contains(err.Error(), "address family not supported")) {
				continue
			}
			return false
		}
		l.Close()
	}
	return true
}

// setup is repeatable: an existing worktree and an already moved PRD stay.
func setup(t Target, r *state.Run, log *logger) error {
	if fsutil.Exists(r.Worktree) {
		log.printf("Worktree %s exists already", r.Worktree)
	} else {
		log.printf("Worktree %s on branch %s from %s", r.Worktree, r.Branch, r.Base)
		if git.BranchExists(t.Dir, r.Branch) {
			return fmt.Errorf("branch %s exists already", r.Branch)
		}
		if err := os.MkdirAll(filepath.Dir(r.Worktree), 0o755); err != nil {
			return err
		}
		if _, err := git.Run(t.Dir, "worktree", "add", "-b", r.Branch, r.Worktree, r.Base); err != nil {
			return err
		}
	}
	if fsutil.Exists(filepath.Join(t.Dir, r.PRD)) && !git.IsTracked(t.Dir, r.PRD) {
		if err := movePRD(t.Dir, r.Worktree, r.PRD, log); err != nil {
			return err
		}
	}
	return runHook(t, r, log, "setup")
}

// movePRD moves an untracked PRD from the main folder into the worktree and commits it there. The
// main folder stays clean, and the PRD reaches the base branch with the merge.
func movePRD(mainDir, wt, rel string, log *logger) error {
	if err := fsutil.Move(filepath.Join(mainDir, rel), filepath.Join(wt, rel)); err != nil {
		return err
	}
	log.printf("PRD %s moved into the worktree", rel)
	if _, err := git.Run(wt, "add", "--", rel); err != nil {
		return err
	}
	msg := "docs(prd): add " + strings.TrimSuffix(filepath.Base(rel), ".md")
	if _, err := git.Run(wt, "commit", "-m", msg, "--", rel); err != nil {
		return err
	}
	log.printf("Commit: %s", msg)
	return nil
}

// hookEnv are the variables hooks and the dev server get.
func hookEnv(t Target, r *state.Run) []string {
	return []string{
		"LAZYDEV_PROJECT=" + t.Dir, "LAZYDEV_WORKTREE=" + r.Worktree, "LAZYDEV_BRANCH=" + r.Branch,
		"LAZYDEV_PORT=" + strconv.Itoa(r.Port), "LAZYDEV_SLUG=" + r.Slug,
	}
}

// scriptPath resolves a script path from config.yaml, relative paths start at the main folder.
func scriptPath(t Target, script string) string {
	if filepath.IsAbs(script) {
		return script
	}
	return filepath.Join(t.Dir, script)
}

// runHook runs the script of a hook from config.yaml in the worktree. Without a script it does
// nothing. config.yaml gets read again for every hook, so trust gets checked again too: a hook that
// changed during a run or before a discard does not run untrusted.
func runHook(t Target, r *state.Run, log *logger, name string) error {
	info := repo.Read(t.Dir, t.ID)
	if info.Err != nil {
		return info.Err
	}
	cfg := info.Config
	script := map[string]string{"setup": cfg.Hooks.Setup, "teardown": cfg.Hooks.Teardown, "check": cfg.Hooks.Check}[name]
	if script == "" {
		return nil
	}
	if info.Trust == repo.TrustPending {
		return fmt.Errorf("%s hook not run, hooks in %s/config.yaml are not trusted", name, repo.Dir)
	}
	script = scriptPath(t, script)
	log.printf("$ %s", script)
	cmd := exec.Command(script)
	cmd.Dir = r.Worktree
	cmd.Env = append(os.Environ(), hookEnv(t, r)...)
	out := log.lines()
	defer out.Close()
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("%s hook ended with exit code %d", name, exit.ExitCode())
		}
		return fmt.Errorf("%s hook: %w", name, err)
	}
	return nil
}

// StartDev starts hooks.dev in the worktree of a run as a detached dev server, with PORT and the
// hook variables.
func StartDev(t Target, prdName string) (int, error) {
	r, err := state.Load(t.ID, prdName)
	if err != nil {
		return 0, err
	}
	if !fsutil.Exists(r.Worktree) {
		return 0, fmt.Errorf("%s has no worktree", prdName)
	}
	if err := t.CheckTrust(); err != nil {
		return 0, err
	}
	cfg, err := t.Repo()
	if err != nil {
		return 0, err
	}
	var command []string
	if cfg.Hooks.Dev != "" {
		command = []string{scriptPath(t, cfg.Hooks.Dev)}
	}
	return devserver.Start(t.ID, prdName, r.Worktree, command, append(hookEnv(t, r), "PORT="+strconv.Itoa(r.Port)))
}

// removeWorktree removes worktree and branch. force also throws away unmerged commits and local
// changes, without it only a clean worktree and a merged branch go.
func removeWorktree(mainDir, path, branch string, force bool) error {
	var errs []error
	if fsutil.Exists(path) {
		args := []string{"worktree", "remove", path}
		if force {
			args = []string{"worktree", "remove", "--force", path}
		}
		if _, err := git.Run(mainDir, args...); err != nil {
			errs = append(errs, err)
		}
	}
	// A folder deleted by hand leaves data in .git/worktrees.
	if _, err := git.Run(mainDir, "worktree", "prune"); err != nil {
		errs = append(errs, err)
	}
	if git.BranchExists(mainDir, branch) {
		flag := "-d"
		if force {
			flag = "-D"
		}
		if _, err := git.Run(mainDir, "branch", flag, branch); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func requireClean(dir, what string) error {
	out, err := git.Run(dir, "status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty := git.Lines(out); len(dirty) > 0 {
		return fmt.Errorf("%s not clean: %s", what, strings.Join(dirty, ", "))
	}
	return nil
}
