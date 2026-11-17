package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lazydev/internal/claude"
	"lazydev/internal/git"
	"lazydev/internal/notify"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// With merge: pr in config.yaml the merge does not fast-forward. It rebases the branch onto
// origin/<base>, runs the check, pushes the branch to origin and opens a pull request with gh. The
// run then waits in phase pr, without taking a slot, until a human merges or closes the pull
// request on GitHub. lazydev never merges a pull request itself.

// GH starts the GitHub CLI. Tests replace it with a fake.
var GH = []string{"gh"}

// PRPollInterval is how often a run in phase pr reads the state of its pull request. Tests shorten
// it.
var PRPollInterval = 60 * time.Second

// The states of a pull request in gh pr view --json state.
const (
	prOpen   = "OPEN"
	prMerged = "MERGED"
	prClosed = "CLOSED"
)

// gh runs the GitHub CLI in dir and returns its stdout. An error carries the first line of stderr.
func gh(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, GH[0], append(slices.Clone(GH[1:]), args...)...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := claude.FirstLine(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gh %s: %s", strings.Join(args[:min(2, len(args))], " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// pullRequest is the part of gh pr view --json lazydev reads.
type pullRequest struct {
	URL        string `json:"url"`
	State      string `json:"state"`
	HeadRefOid string `json:"headRefOid"`
}

// viewPullRequest reads the fields of the pull request ref, a URL or a branch.
func viewPullRequest(ctx context.Context, dir, ref, fields string) (pullRequest, error) {
	out, err := gh(ctx, dir, "pr", "view", ref, "--json", fields)
	if err != nil {
		return pullRequest{}, err
	}
	var pr pullRequest
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return pullRequest{}, fmt.Errorf("gh pr view: %w", err)
	}
	return pr, nil
}

// ghReady makes sure gh exists and is logged in.
func ghReady(ctx context.Context, dir string) error {
	if _, err := exec.LookPath(GH[0]); err != nil {
		return errors.New("gh not found, install the GitHub CLI for merge: pr")
	}
	if _, err := gh(ctx, dir, "auth", "status"); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("gh is not authenticated, run gh auth login")
	}
	return nil
}

// mergePullRequest is the merge of merge: pr: it publishes the branch as a pull request and waits
// for it. A run whose pull request is still the current one waits again right away.
func mergePullRequest(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) error {
	if err := ghReady(ctx, t.Dir); err != nil {
		return err
	}
	if !resumePullRequest(ctx, t, r, log) {
		if err := publishPullRequest(ctx, t, prdName, r, log); err != nil {
			return err
		}
	}
	return waitPullRequest(ctx, t, prdName, r, log)
}

// resumePullRequest reports whether the run may wait for its pull request again without rebase and
// push: the pull request is not closed and its head is the head of the branch.
func resumePullRequest(ctx context.Context, t Target, r *state.Run, log *logger) bool {
	if r.PullRequest == "" {
		return false
	}
	pr, err := viewPullRequest(ctx, t.Dir, r.PullRequest, "state,headRefOid")
	if err != nil {
		log.printf("Pull request %s not readable, merging again: %v", r.PullRequest, err)
		return false
	}
	head, err := git.Run(r.Worktree, "rev-parse", "HEAD")
	if err != nil || pr.State == prClosed || pr.HeadRefOid != head {
		return false
	}
	log.printf("Pull request %s is current, waiting for it again", r.PullRequest)
	return true
}

// publishPullRequest rebases the branch onto origin/<base>, checks it, records cost and model,
// pushes it and opens or updates its pull request. It holds the merge slot only for that.
func publishPullRequest(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) error {
	unlock, err := beginMerge(ctx, t, prdName, r, log)
	if err != nil {
		return err
	}
	defer unlock()
	log.printf("$ git fetch origin %s", r.Base)
	if _, err := git.Run(r.Worktree, "fetch", "origin", r.Base); err != nil {
		return err
	}
	if err := rebaseAndCheck(ctx, t, prdName, r, "origin/"+r.Base, log); err != nil {
		return err
	}
	if err := recordRunInfo(prdName, r, log); err != nil {
		return err
	}
	log.printf("$ git push --force-with-lease -u origin %s", r.Branch)
	// A merge after an error pushes a rebased branch, so the push may replace the remote branch.
	if _, err := git.Run(r.Worktree, "push", "--force-with-lease", "-u", "origin", r.Branch); err != nil {
		return err
	}
	url, err := openPullRequest(ctx, t, prdName, r, log)
	if err != nil {
		return err
	}
	r.PullRequest = url
	return state.Save(t.ID, prdName, r)
}

// openPullRequest takes the open pull request of the branch and sets its title and description, or
// creates one. It returns the URL.
func openPullRequest(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) (string, error) {
	title, body, err := pullRequestText(t, prdName, r)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "lazydev-pr-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	// Without a pull request of the branch gh fails, then lazydev creates one.
	if pr, err := viewPullRequest(ctx, r.Worktree, r.Branch, "url,state"); err == nil && pr.State == prOpen && pr.URL != "" {
		log.printf("$ gh pr edit %s --title %s", pr.URL, title)
		if _, err := gh(ctx, r.Worktree, "pr", "edit", pr.URL, "--title", title, "--body-file", f.Name()); err != nil {
			return "", err
		}
		log.printf("Pull request %s updated", pr.URL)
		return pr.URL, nil
	}
	log.printf("$ gh pr create --base %s --head %s --title %s", r.Base, r.Branch, title)
	out, err := gh(ctx, r.Worktree, "pr", "create", "--base", r.Base, "--head", r.Branch, "--title", title, "--body-file", f.Name())
	if err != nil {
		return "", err
	}
	lines := git.Lines(out)
	if len(lines) == 0 {
		return "", errors.New("gh pr create printed no URL")
	}
	url := strings.TrimSpace(lines[len(lines)-1])
	log.printf("Pull request %s created", url)
	return url, nil
}

// pullRequestText is title and description of the pull request: from pr.md of the evidence folder,
// which review writes, otherwise the title of the PRD and the detail of its status line.
func pullRequestText(t Target, prdName string, r *state.Run) (title, body string, err error) {
	title, body, err = readPullRequestFile(filepath.Join(state.EvidenceDir(t.ID, prdName), evidencePR))
	if err == nil {
		return title, body, nil
	}
	data, err := os.ReadFile(filepath.Join(r.Worktree, r.PRD))
	if err != nil {
		return "", "", err
	}
	title = prdName
	for line := range strings.SplitSeq(string(data), "\n") {
		if h, ok := strings.CutPrefix(strings.TrimSpace(line), "# "); ok {
			title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(h), "PRD:"))
			break
		}
	}
	var scope []string
	if d, err := project.Read(r.Worktree, filepath.Join(r.Worktree, r.PRD)); err == nil {
		first, rest, _ := strings.Cut(d.Status.Detail, "\n")
		// Cost and model are no scope.
		for part := range strings.SplitSeq(first, "·") {
			if part = strings.TrimSpace(part); part != "" && !project.IsRunInfo(part) {
				scope = append(scope, part)
			}
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			scope = append(scope, rest)
		}
	}
	body = strings.Join(scope, " · ")
	if body != "" {
		body += "\n\n"
	}
	return title, body + "PRD: `" + r.PRD + "`\n", nil
}

// readPullRequestFile reads pr.md: the first line "# <title>", then the description.
func readPullRequestFile(path string) (title, body string, err error) {
	data, err := os.ReadFile(path)
	if err == nil {
		first, rest, _ := strings.Cut(strings.TrimLeft(string(data), "\r\n"), "\n")
		if h, ok := strings.CutPrefix(strings.TrimSpace(first), "# "); ok && strings.TrimSpace(h) != "" {
			return strings.TrimSpace(h), strings.TrimSpace(rest) + "\n", nil
		}
	}
	return "", "", fmt.Errorf("review wrote no pull request description to %s", path)
}

// waitPullRequest puts the run into phase pr and reads the state of its pull request every
// PRPollInterval until it is merged or closed. The merge slot is free meanwhile, and the run takes
// no runner slot, so waiting PRDs start right away.
func waitPullRequest(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) error {
	log.printf("Pull request %s", r.PullRequest)
	r.Phase, r.Note = project.PhasePR, ""
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	notifyRun(t, r, notify.PR, log)
	if err := startWaiting(); err != nil {
		log.printf("Waiting PRDs not started: %v", err)
	}
	unreadable := false
	for {
		pr, err := viewPullRequest(ctx, t.Dir, r.PullRequest, "state")
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch {
		case err != nil && !unreadable:
			// One log line per streak of errors, the runner keeps waiting.
			log.printf("PR state not readable: %s", claude.FirstLine(err.Error()))
			r.Note, unreadable = "pull request state not readable", true
			if err := state.Save(t.ID, prdName, r); err != nil {
				return err
			}
		case err == nil && unreadable:
			log.printf("PR state readable again")
			r.Note, unreadable = "", false
			if err := state.Save(t.ID, prdName, r); err != nil {
				return err
			}
		}
		if err == nil {
			switch pr.State {
			case prMerged:
				return finishPullRequest(ctx, t, prdName, r, log)
			case prClosed:
				return fmt.Errorf("pull request %s closed without merge", r.PullRequest)
			}
		}
		if err := sleep(ctx, PRPollInterval); err != nil {
			return err
		}
	}
}

// finishPullRequest cleans up after the merge of the pull request like after a fast-forward and
// brings the main folder up to origin/<base> if it is on the base branch and clean.
func finishPullRequest(ctx context.Context, t Target, prdName string, r *state.Run, log *logger) error {
	log.printf("Pull request %s merged", r.PullRequest)
	unlock, err := waitMergeLock(ctx, t.ID, log)
	if err != nil {
		return err
	}
	defer unlock()
	r.Phase = project.PhaseMerge
	if err := state.Save(t.ID, prdName, r); err != nil {
		return err
	}
	var notes []string
	// GitHub may squash or rebase, then the branch is no ancestor of the base branch.
	if note := cleanupMerged(t, r, log, true); note != "" {
		notes = append(notes, note)
	}
	if problem := updateMainFolder(t.Dir, r.Base, log); problem != "" {
		log.printf("Main folder not updated: %s", problem)
		notes = append(notes, "main folder not updated: "+problem)
	}
	r.Note = strings.Join(notes, "; ")
	return nil
}

// updateMainFolder fetches the base branch into the main folder and fast-forwards it to
// origin/<base> when nothing blocks it, like the fast-forward of a merge. It returns what blocked it,
// empty on success. It does not wait.
func updateMainFolder(dir, base string, log *logger) string {
	log.printf("$ git fetch origin %s", base)
	if _, err := git.Run(dir, "fetch", "origin", base); err != nil {
		return err.Error()
	}
	problem, err := mainDirProblem(dir, base)
	if err != nil {
		return err.Error()
	}
	if problem != "" {
		return problem
	}
	log.printf("$ git merge --ff-only origin/%s", base)
	if _, err := git.Run(dir, "merge", "--ff-only", "origin/"+base); err != nil {
		return err.Error()
	}
	log.printf("%s is at origin/%s", base, base)
	return ""
}

// closePullRequest closes the open pull request of a discarded run and deletes its remote branch.
func closePullRequest(dir, url string) error {
	ctx := context.Background()
	pr, err := viewPullRequest(ctx, dir, url, "state")
	if err != nil {
		return err
	}
	if pr.State != prOpen {
		return nil
	}
	_, err = gh(ctx, dir, "pr", "close", url, "--delete-branch")
	return err
}
