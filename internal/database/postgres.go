package database

import (
	"fmt"
	"strings"
	"time"

	"lazydev/internal/docker"
)

// maxNameLen is the Postgres limit for identifiers in bytes.
const maxNameLen = 63

// pollInterval is how often WaitReady asks pg_isready. Tests shorten it.
var pollInterval = time.Second

// Target is a database in a running Postgres container.
type Target struct {
	Container, User, Name string
}

// Exists asks pg_database for the name. The name only goes into SQL as a string literal with
// doubled quotes.
func Exists(t Target) (bool, error) {
	query := "SELECT 1 FROM pg_database WHERE datname = '" + strings.ReplaceAll(t.Name, "'", "''") + "'"
	out, err := docker.Exec(t.Container, "psql", "-U", t.User, "-d", "postgres", "-tAc", query)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}

// Create creates the database. An existing one stays, so a repeated setup goes on after an error.
func Create(t Target, log Logger) error {
	exists, err := Exists(t)
	if err != nil {
		return err
	}
	if exists {
		log("Database %s in %s exists already", t.Name, t.Container)
		return nil
	}
	if _, err := docker.Exec(t.Container, "createdb", "-U", t.User, t.Name); err != nil {
		return err
	}
	log("Database %s in %s created", t.Name, t.Container)
	return nil
}

// Drop drops the database of the worktree with --force, which ends open connections, e.g. of a
// forgotten dev server. It only drops names ending in "_<slug>", so the main databases stay.
func Drop(t Target, slug string, log Logger) error {
	if slug == "" || !strings.HasSuffix(t.Name, "_"+slug) {
		return fmt.Errorf("database %s does not belong to the worktree, not dropping it", t.Name)
	}
	if _, err := docker.Exec(t.Container, "dropdb", "--if-exists", "--force", "-U", t.User, t.Name); err != nil {
		return err
	}
	log("Database %s in %s dropped", t.Name, t.Container)
	return nil
}

// WaitReady waits until pg_isready succeeds in the container, at most timeout.
func WaitReady(container, user string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		_, err := docker.Exec(container, "pg_isready", "-q", "-U", user)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(pollInterval)
	}
}
