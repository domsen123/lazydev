package tui

import (
	"path/filepath"

	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/state"
	"lazydev/internal/terminal"
)

// projectEntry is a project of the left column. Each project has its own terminal tabs, which keep
// running while another project is selected.
type projectEntry struct {
	path  string
	terms []*terminal.Terminal
	tab   int
	git   gitInfo
	// tree is the file tree of the right column, created when first shown.
	tree *fileTree
	// dockerCursor is the selected service in the docker tab.
	dockerCursor int
	// lazydev is the lazydev setup for the details tab, read with the git rhythm.
	lazydev repo.Info
	// prds are the PRDs of the right column, read with the git rhythm. prdCursor indexes
	// prdRows, showDone unfolds the done PRDs.
	prds   []project.PRD
	prdErr error
	// runs maps PRD names to their runs, read every second.
	runs      map[string]*state.Run
	prdCursor int
	showDone  bool
}

func (p *projectEntry) name() string {
	return filepath.Base(p.path)
}

// files returns the file tree of the project and creates it on first use.
func (p *projectEntry) files() *fileTree {
	if p.tree == nil {
		p.tree = newFileTree(p.path)
	}
	return p.tree
}

// loadProjects reads the project list.
func loadProjects() ([]*projectEntry, error) {
	paths, err := config.LoadProjects()
	projects := make([]*projectEntry, len(paths))
	for i, p := range paths {
		projects[i] = &projectEntry{path: p}
	}
	return projects, err
}

// saveProjects writes the project list.
func saveProjects(projects []*projectEntry) error {
	return config.SaveProjects(projectPaths(projects))
}

func projectPaths(projects []*projectEntry) []string {
	paths := make([]string, len(projects))
	for i, p := range projects {
		paths[i] = p.path
	}
	return paths
}

// projectIDs are the IDs of the projects, see config.IDs.
func projectIDs(projects []*projectEntry) []string {
	return config.IDs(projectPaths(projects))
}
