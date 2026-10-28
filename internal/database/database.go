// Package database gives every worktree its own empty Postgres database in a compose service of
// the project. The name is the one from the env file of the main folder plus "_<slug>", and the
// copy of the env file in the worktree names it. Migrations stay in the setup hook of the project.
package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lazydev/internal/docker"
	"lazydev/internal/fsutil"
	"lazydev/internal/git"
	"lazydev/internal/repo"
	"lazydev/internal/state"
)

// ReadyTimeout limits the wait for Postgres in a container. Tests shorten it.
var ReadyTimeout = 60 * time.Second

// defaultUser applies without POSTGRES_USER in the compose service.
const defaultUser = "postgres"

// Logger writes a runner line into the step log.
type Logger func(format string, args ...any)

// Planned is an entry of databases with everything the setup needs.
type Planned struct {
	repo.Database
	User, Name string
	// EnvContent is the new content of the env file in the worktree, with the keys of all entries
	// of that file replaced.
	EnvContent string
}

// Plan computes per entry the compose user, the database name "<base name>_<slug>" and the new
// content of its env file in the worktree.
func Plan(projectDir, slug string, dbs []repo.Database) ([]Planned, error) {
	// main holds the env files of the main folder, out their worktree copies.
	main, copies := map[string]string{}, map[string]string{}
	users := userCache{dir: projectDir}
	var out []Planned
	for _, db := range dbs {
		file := db.Env.File
		if _, ok := main[file]; !ok {
			data, err := os.ReadFile(filepath.Join(projectDir, file))
			if err != nil {
				return nil, fmt.Errorf("%s missing in the main folder", file)
			}
			main[file], copies[file] = string(data), string(data)
		}
		value := parseEnv(main[file])[db.Env.Key]
		name, err := worktreeName(db, value, slug)
		if err != nil {
			return nil, err
		}
		user, err := users.user(db)
		if err != nil {
			return nil, err
		}
		copies[file] = setEnv(copies[file], db.Env.Key, ReplaceName(value, name))
		out = append(out, Planned{Database: db, User: user, Name: name})
	}
	for i := range out {
		out[i].EnvContent = copies[out[i].Env.File]
	}
	return out, nil
}

// worktreeName is "<base name>_<slug>" for the value of the key.
func worktreeName(db repo.Database, value, slug string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s: %s is not set", db.Env.File, db.Env.Key)
	}
	base, err := BaseName(value)
	if errors.Is(err, errNoName) {
		return "", fmt.Errorf("%s: %s has no database name", db.Env.File, db.Env.Key)
	} else if err != nil {
		return "", fmt.Errorf("%s: %s: %w", db.Env.File, db.Env.Key, err)
	}
	name := base + "_" + slug
	if len(name) > maxNameLen {
		return "", fmt.Errorf("database name %s is longer than %d bytes", name, maxNameLen)
	}
	return name, nil
}

// Prepare writes the env files into the worktree, starts missing containers and creates the
// databases. Existing databases stay. It returns the databases once per service and name.
func Prepare(projectDir, worktree, slug string, dbs []repo.Database, log Logger) ([]state.Database, error) {
	return create(projectDir, worktree, slug, dbs, false, log)
}

// Recreate drops the databases and creates them empty again, e.g. after a rebase changed the
// migrations.
func Recreate(projectDir, worktree, slug string, dbs []repo.Database, log Logger) ([]state.Database, error) {
	return create(projectDir, worktree, slug, dbs, true, log)
}

func create(projectDir, worktree, slug string, dbs []repo.Database, recreate bool, log Logger) ([]state.Database, error) {
	planned, err := Plan(projectDir, slug, dbs)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, p := range planned {
		if !slices.Contains(written, p.Env.File) {
			if _, err := git.Run(worktree, "check-ignore", "-q", p.Env.File); err != nil {
				return nil, fmt.Errorf("%s is not ignored by git, a worktree copy would dirty the worktree", p.Env.File)
			}
			if err := fsutil.WriteAtomic(filepath.Join(worktree, p.Env.File), []byte(p.EnvContent), 0o600); err != nil {
				return nil, err
			}
			written = append(written, p.Env.File)
		}
		log("%s: %s → %s", p.Env.File, p.Env.Key, p.Name)
	}
	containers := map[string]string{}
	var out []state.Database
	for _, p := range planned {
		db := state.Database{Service: p.Service, Name: p.Name}
		if slices.Contains(out, db) {
			continue
		}
		key := p.Compose + "\x00" + p.Service
		container, ok := containers[key]
		if !ok {
			if container, err = ensureContainer(projectDir, p, log); err != nil {
				return nil, err
			}
			containers[key] = container
		}
		t := Target{Container: container, User: p.User, Name: p.Name}
		if recreate {
			if err := Drop(t, slug, log); err != nil {
				return nil, err
			}
		}
		if err := Create(t, log); err != nil {
			return nil, err
		}
		out = append(out, db)
	}
	return out, nil
}

// ensureContainer finds the running container of the service and starts it with docker compose
// up -d if it does not run. It waits until Postgres accepts connections.
func ensureContainer(projectDir string, p Planned, log Logger) (string, error) {
	dir := composeDir(projectDir, p.Compose)
	container, ok, err := docker.RunningContainer(dir, p.Service)
	if err != nil {
		return "", err
	}
	notReady := fmt.Errorf("postgres in service %s not ready after %ds", p.Service, int(ReadyTimeout.Seconds()))
	if !ok {
		log("Starting service %s: docker %s", p.Service, strings.Join(docker.UpArgs(p.Compose, p.Service), " "))
		if err := docker.Up(projectDir, p.Compose, p.Service); err != nil {
			return "", err
		}
		if container, ok, err = docker.RunningContainer(dir, p.Service); err != nil {
			return "", err
		} else if !ok {
			return "", notReady
		}
	}
	if err := WaitReady(container, p.User, ReadyTimeout); err != nil {
		return "", notReady
	}
	return container, nil
}

// DropAll drops the databases of the worktree without starting a container. Without databases of
// the run state it computes them from the entries and the env files of the main folder.
func DropAll(projectDir, slug string, dbs []state.Database, entries []repo.Database, log Logger) error {
	var errs []error
	if len(dbs) == 0 {
		for _, e := range entries {
			data, err := os.ReadFile(filepath.Join(projectDir, e.Env.File))
			var base string
			if err == nil {
				base, err = BaseName(parseEnv(string(data))[e.Env.Key])
			}
			if err != nil || base == "" {
				errs = append(errs, fmt.Errorf("%s: database not determinable", e.Env.File))
				continue
			}
			if db := (state.Database{Service: e.Service, Name: base + "_" + slug}); !slices.Contains(dbs, db) {
				dbs = append(dbs, db)
			}
		}
	}
	users := userCache{dir: projectDir}
	for _, db := range dbs {
		entry := repo.Database{Service: db.Service}
		if i := slices.IndexFunc(entries, func(e repo.Database) bool { return e.Service == db.Service }); i >= 0 {
			entry = entries[i]
		}
		container, ok, err := docker.RunningContainer(composeDir(projectDir, entry.Compose), db.Service)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !ok {
			log("Service %s is not running, database %s stays", db.Service, db.Name)
			continue
		}
		user, err := users.user(entry)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := Drop(Target{Container: container, User: user, Name: db.Name}, slug, log); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// composeDir is the folder of the compose project: the folder of the compose file, the main folder
// for the default file. Docker Compose writes it into the label working_dir.
func composeDir(projectDir, compose string) string {
	if compose == "" {
		return projectDir
	}
	return filepath.Dir(filepath.Join(projectDir, compose))
}

// userCache reads the compose files once each and finds POSTGRES_USER of a service.
type userCache struct {
	dir   string
	files map[string]map[string]docker.ComposeService
}

func (c *userCache) user(db repo.Database) (string, error) {
	if c.files == nil {
		c.files = map[string]map[string]docker.ComposeService{}
	}
	services, ok := c.files[db.Compose]
	if !ok {
		var err error
		if services, err = docker.ComposeConfig(c.dir, db.Compose); err != nil {
			return "", err
		}
		c.files[db.Compose] = services
	}
	if user := services[db.Service].Environment["POSTGRES_USER"]; user != "" {
		return user, nil
	}
	return defaultUser, nil
}
