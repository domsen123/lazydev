// Package project liest Projekte und deren PRDs von der Platte.
package project

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PRDDir ist der Ordner der PRDs relativ zum Projekt.
const PRDDir = "prd"

type Project struct {
	ID   string
	Path string
	// Branch ist der aktuelle Branch des Hauptordners.
	Branch string
	PRDs   []PRD
	// Err beschreibt, warum das Projekt nicht vollständig lesbar ist.
	Err error
}

func (p Project) Name() string {
	return filepath.Base(p.Path)
}

type PRD struct {
	// Path ist relativ zum Projekt, z. B. "prd/auth/mobile-bearer-token-auth.md".
	Path string
	// Name ist der Dateiname ohne ".md".
	Name string
	// Area ist der Bereich, z. B. "auth".
	Area   string
	Title  string
	Status Status
	// Tracked ist false, solange das PRD im Hauptordner ungetrackt liegt.
	Tracked bool
	// Branch ist "feat/<name>", wenn der Branch existiert. Sonst "main" für gemergte
	// PRDs und leer für alle anderen.
	Branch string
	// Created ist der frühere Zeitpunkt aus Erstellzeit der Datei und erstem Commit.
	Created time.Time
}

// Label ist "<bereich>/<name>", wie die PRD-Liste es zeigt.
func (d PRD) Label() string {
	if d.Area == "" {
		return d.Name
	}
	return d.Area + "/" + d.Name
}

func (d PRD) Phase() Phase {
	return PhaseFromStatus(d.Status)
}

// Load liest ein Projekt. Fehler landen in Project.Err, damit die Liste das Projekt
// trotzdem zeigt.
func Load(id, path string) Project {
	p := Project{ID: id, Path: path}
	if info, err := os.Stat(path); err != nil {
		p.Err = err
		return p
	} else if !info.IsDir() {
		p.Err = errors.New("kein Ordner")
		return p
	}

	prds, err := scanPRDs(path)
	if err != nil {
		p.Err = err
	}

	branch, gitErr := currentBranch(path)
	if gitErr != nil {
		p.Err = errors.Join(p.Err, gitErr)
		p.PRDs = prds
		return p
	}
	p.Branch = branch

	untracked, _ := untrackedFiles(path, PRDDir)
	features, _ := featureBranches(path)
	added := addedTimes(path, PRDDir)
	for i := range prds {
		d := &prds[i]
		d.Tracked = !untracked[d.Path]
		if at, ok := added[d.Path]; ok && (d.Created.IsZero() || at.Before(d.Created)) {
			d.Created = at
		}
		switch fb := FeatureBranchPrefix + d.Name; {
		case features[fb]:
			d.Branch = fb
		case d.Status.Kind == StatusImplemented:
			d.Branch = MainBranch
		}
	}
	p.PRDs = prds
	return p
}

func scanPRDs(root string) ([]PRD, error) {
	dir := filepath.Join(root, PRDDir)
	var prds []PRD
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		d, err := ReadPRD(root, path)
		if err != nil {
			return err
		}
		prds = append(prds, d)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	sort.Slice(prds, func(i, j int) bool { return prds[i].Path < prds[j].Path })
	return prds, err
}

// ReadPRD liest ein PRD unter root, etwa im Hauptordner oder in einem Worktree.
func ReadPRD(root, path string) (PRD, error) {
	f, err := os.Open(path)
	if err != nil {
		return PRD{}, err
	}
	defer f.Close()
	title, status, err := ParseHeader(f)
	if err != nil {
		return PRD{}, err
	}
	rel, _ := filepath.Rel(root, path)
	rel = filepath.ToSlash(rel)
	area := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(rel, PRDDir+"/")))
	if area == "." {
		area = ""
	}
	return PRD{
		Path:    rel,
		Name:    strings.TrimSuffix(filepath.Base(path), ".md"),
		Area:    area,
		Title:   title,
		Status:  status,
		Created: fileCreated(path),
	}, nil
}

// FindPRD sucht ein PRD über seinen Namen, also den Dateinamen ohne ".md".
func (p Project) FindPRD(name string) (PRD, bool) {
	for _, d := range p.PRDs {
		if d.Name == name {
			return d, true
		}
	}
	return PRD{}, false
}

// AssignIDs vergibt die Projekt-ID aus dem Ordnernamen. Teilen sich mehrere Projekte
// einen Ordnernamen, bekommen alle davon einen kurzen Hash des Pfads angehängt.
// So bleibt die ID eines Projekts stabil, egal in welcher Reihenfolge es eingetragen ist.
func AssignIDs(paths []string) []string {
	count := map[string]int{}
	for _, p := range paths {
		count[filepath.Base(p)]++
	}
	ids := make([]string, len(paths))
	for i, p := range paths {
		base := filepath.Base(p)
		if count[base] > 1 {
			sum := sha1.Sum([]byte(p))
			base += "-" + hex.EncodeToString(sum[:])[:6]
		}
		ids[i] = base
	}
	return ids
}
