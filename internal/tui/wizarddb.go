package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lazydev/internal/database"
	"lazydev/internal/docker"
	"lazydev/internal/git"
	"lazydev/internal/repo"
)

// The page Databases of the wizard lists the Postgres services of the compose files with their
// entries of databases. a adds an entry through a choice of env file and key, d removes one.

// dbService is a row of a service on the page Databases.
type dbService struct {
	name, image string
	// compose is the compose file the service comes from, empty for the default file.
	compose string
	// missing is true for a service of config.yaml that no compose file has as Postgres service.
	missing bool
}

// dbRow is a line of the page: a service, or with entry >= 0 an entry of databases below it.
type dbRow struct {
	service, entry int
}

// dbPick is the choice of env file and key while adding an entry.
type dbPick struct {
	service int
	// file is empty while the env file is chosen, then the key follows.
	file    string
	options []string
	cursor  int
}

const (
	dbNoCompose  = "No compose.yaml or docker-compose.yml in the main folder. Set compose: in config.yaml for other files."
	dbNoPostgres = "No Postgres service in the compose file."
	dbNoEnvFile  = "No ignored .env file in the main folder."
)

// loadDatabases reads the Postgres services of the default compose file and of every compose file
// of the entries, and the entries of config.yaml.
func (w *wizard) loadDatabases(dbs []repo.Database) {
	w.databases = slices.Clone(dbs)
	files := []string{""}
	for _, db := range dbs {
		if !slices.Contains(files, db.Compose) {
			files = append(files, db.Compose)
		}
	}
	found := false
	for _, file := range files {
		if file == "" && !docker.HasCompose(w.path) {
			w.dbMessage = dbNoCompose
			continue
		}
		services, err := docker.ComposeConfig(w.path, file)
		if err != nil {
			if w.dbMessage == "" {
				w.dbMessage, _, _ = strings.Cut(err.Error(), "\n")
			}
			continue
		}
		names := make([]string, 0, len(services))
		for name := range services {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if s := services[name]; docker.IsPostgres(s) {
				w.dbServices = append(w.dbServices, dbService{name: name, image: s.Image, compose: file})
				found = true
			}
		}
	}
	if !found && w.dbMessage == "" {
		w.dbMessage = dbNoPostgres
	}
	for _, db := range dbs {
		if w.serviceIndex(db) < 0 {
			w.dbServices = append(w.dbServices, dbService{name: db.Service, compose: db.Compose, missing: true})
		}
	}
}

// serviceIndex is the row of the service of db, -1 without one.
func (w *wizard) serviceIndex(db repo.Database) int {
	return slices.IndexFunc(w.dbServices, func(s dbService) bool { return s.name == db.Service && s.compose == db.Compose })
}

// dbRows are the lines of the page: each service, below it its entries in list order.
func (w *wizard) dbRows() []dbRow {
	var rows []dbRow
	for i := range w.dbServices {
		rows = append(rows, dbRow{service: i, entry: -1})
		for j, db := range w.databases {
			if w.serviceIndex(db) == i {
				rows = append(rows, dbRow{service: i, entry: j})
			}
		}
	}
	return rows
}

// databaseList is databases in the order of the page.
func (w *wizard) databaseList() []repo.Database {
	var out []repo.Database
	for _, r := range w.dbRows() {
		if r.entry >= 0 {
			out = append(out, w.databases[r.entry])
		}
	}
	return out
}

// updateDatabasesPage handles the keys of the page Databases and of its choices.
func (m *model) updateDatabasesPage(key string) {
	w := m.wizard
	if p := w.dbPick; p != nil {
		switch key {
		case "j", "down":
			p.cursor = clamp(p.cursor+1, len(p.options))
		case "k", "up":
			p.cursor = clamp(p.cursor-1, len(p.options))
		case "enter":
			w.choose()
		}
		return
	}
	rows := w.dbRows()
	switch key {
	case "j", "down":
		w.dbCursor = clamp(w.dbCursor+1, len(rows))
	case "k", "up":
		w.dbCursor = clamp(w.dbCursor-1, len(rows))
	case "a":
		if w.dbCursor < len(rows) && rows[w.dbCursor].entry < 0 {
			if msg := w.startPick(rows[w.dbCursor].service); msg != "" {
				m.err = msg
			}
		}
	case "d":
		w.removeEntry()
	case "enter":
		w.page = pagePrompts
	case "backspace":
		w.page = w.lastChoicePage()
	}
}

// startPick opens the choice of the env file for the service of row. It returns an error text
// without an ignored env file.
func (w *wizard) startPick(service int) string {
	files := envFiles(w.path)
	if len(files) == 0 {
		return dbNoEnvFile
	}
	w.dbPick = &dbPick{service: service, options: files}
	return ""
}

// choose takes the selected option: first the env file, then the key, which adds the entry.
func (w *wizard) choose() {
	p := w.dbPick
	if len(p.options) == 0 {
		return
	}
	choice := p.options[p.cursor]
	if p.file == "" {
		data, _ := os.ReadFile(filepath.Join(w.path, choice))
		w.dbPick = &dbPick{service: p.service, file: choice, options: sortKeys(database.Keys(string(data)))}
		return
	}
	s := w.dbServices[p.service]
	db := repo.Database{Service: s.name, Compose: s.compose, Env: repo.DatabaseEnv{File: p.file, Key: choice}}
	if !slices.Contains(w.databases, db) {
		w.databases = append(w.databases, db)
	}
	w.dbPick = nil
}

// removeEntry drops the entry of the selected row.
func (w *wizard) removeEntry() {
	rows := w.dbRows()
	if w.dbCursor >= len(rows) || rows[w.dbCursor].entry < 0 {
		return
	}
	w.databases = slices.Delete(w.databases, rows[w.dbCursor].entry, rows[w.dbCursor].entry+1)
	w.dbCursor = clamp(w.dbCursor, len(w.dbRows()))
}

// envFiles are the regular files in dir whose name starts with .env and that git ignores, sorted.
func envFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".env") || !e.Type().IsRegular() {
			continue
		}
		if _, err := git.Run(dir, "check-ignore", "-q", e.Name()); err == nil {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// sortKeys puts keys naming a database first, otherwise the file order stays.
func sortKeys(keys []string) []string {
	likely := func(k string) bool {
		k = strings.ToUpper(k)
		return strings.Contains(k, "DATABASE") || strings.Contains(k, "DB_NAME") || strings.Contains(k, "_URL")
	}
	var first, rest []string
	for _, k := range keys {
		if likely(k) {
			first = append(first, k)
		} else {
			rest = append(rest, k)
		}
	}
	return append(first, rest...)
}
