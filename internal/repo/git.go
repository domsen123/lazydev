package repo

import (
	"slices"
	"strings"

	"lazydev/internal/git"
	"lazydev/internal/project"
)

// LocalBranches lists the local branches without lazydev's PRD branches.
func LocalBranches(dir string) ([]string, error) {
	out, err := git.Run(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, b := range git.Lines(out) {
		if !strings.HasPrefix(b, project.FeaturePrefix) {
			branches = append(branches, b)
		}
	}
	return branches, nil
}

// KnowledgeCandidates lists the tracked .md files outside prd/ and .lazydev/, sorted. Only tracked
// files exist in a worktree too.
func KnowledgeCandidates(dir string) ([]string, error) {
	out, err := git.Run(dir, "ls-files", "--", "*.md")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range git.Lines(out) {
		if strings.HasPrefix(f, project.Dir+"/") || strings.HasPrefix(f, Dir+"/") {
			continue
		}
		files = append(files, f)
	}
	slices.Sort(files)
	return files, nil
}

// DetectBaseBranch picks main or master if the repository has one, otherwise the checked out
// branch. Outside a repository it is empty, which means main.
func DetectBaseBranch(dir string) string {
	for _, b := range []string{"main", "master"} {
		if git.BranchExists(dir, b) {
			return b
		}
	}
	return git.CurrentBranch(dir)
}
