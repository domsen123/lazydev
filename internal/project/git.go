package project

import (
	"lazydev/internal/git"
)

// FeatureBranchPrefix ergibt mit dem PRD-Namen den Branch "feat/<prd-name>".
const FeatureBranchPrefix = "feat/"

// MainBranch ist der Branch, von dem Worktrees abzweigen und in den lazydev mergt.
const MainBranch = "main"

// currentBranch liefert bei einem losgelösten HEAD den Kurz-Hash.
func currentBranch(dir string) (string, error) {
	if b, err := git.Run(dir, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		return b, nil
	}
	return git.Run(dir, "rev-parse", "--short", "HEAD")
}

// featureBranches liefert die lokalen Branches unter feat/.
func featureBranches(dir string) (map[string]bool, error) {
	out, err := git.Run(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+FeatureBranchPrefix)
	if err != nil {
		return nil, err
	}
	return lineSet(out), nil
}

// untrackedFiles liefert ungetrackte, nicht ignorierte Dateien unter dir/sub.
func untrackedFiles(dir, sub string) (map[string]bool, error) {
	out, err := git.Run(dir, "ls-files", "--others", "--exclude-standard", "--", sub)
	if err != nil {
		return nil, err
	}
	return lineSet(out), nil
}

func lineSet(out string) map[string]bool {
	set := map[string]bool{}
	for _, l := range git.Lines(out) {
		set[l] = true
	}
	return set
}
