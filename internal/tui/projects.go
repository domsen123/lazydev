package tui

import (
	"path/filepath"
	"slices"

	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/repo"
	"lazydev/internal/state"
	"lazydev/internal/terminal"
)

// projectEntry is a project of the left column. Each project has its own center tabs, whose
// terminals keep running while another project is selected.
type projectEntry struct {
	path string
	tabs []centerTab
	tab  int
	git  gitInfo
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
	// runCursor is the selected run in the run list of the details tab.
	runCursor int
}

func (p *projectEntry) name() string {
	return filepath.Base(p.path)
}

// terminals are the terminal tabs of the project.
func (p *projectEntry) terminals() []*terminal.Terminal {
	var terms []*terminal.Terminal
	for _, t := range p.tabs {
		if term, ok := t.(*terminal.Terminal); ok {
			terms = append(terms, term)
		}
	}
	return terms
}

// activeTerminal is the active tab if it is a terminal, nil otherwise.
func (p *projectEntry) activeTerminal() *terminal.Terminal {
	if p.tab >= len(p.tabs) {
		return nil
	}
	term, _ := p.tabs[p.tab].(*terminal.Terminal)
	return term
}

// activeLog is the active tab if it is a log tab, nil otherwise.
func (p *projectEntry) activeLog() *logView {
	if p.tab >= len(p.tabs) {
		return nil
	}
	lv, _ := p.tabs[p.tab].(*logView)
	return lv
}

// activeView is the active tab if it is a viewer tab, nil otherwise.
func (p *projectEntry) activeView() *mdView {
	if p.tab >= len(p.tabs) {
		return nil
	}
	v, _ := p.tabs[p.tab].(*mdView)
	return v
}

// removeTab drops tab i. The active tab stays the same or moves to its left neighbor.
func (p *projectEntry) removeTab(i int) {
	p.tabs = slices.Delete(p.tabs, i, i+1)
	if p.tab >= i && p.tab > 0 {
		p.tab--
	}
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
