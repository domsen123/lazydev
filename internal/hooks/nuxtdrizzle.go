package hooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lazydev/internal/docker"
	"lazydev/internal/git"
)

// envFile beschreibt eine .env-Datei des Presets nuxt-drizzle und den Container dahinter.
type envFile struct {
	name string
	// startScript startet den Container im Hauptordner, falls er nicht läuft.
	startScript string
	// setPort schreibt PORT für den Dev-Server des Worktrees.
	setPort bool
}

var nuxtDrizzleEnvFiles = []envFile{
	{name: ".env", startScript: "docker:up", setPort: true},
	{name: ".env.test", startScript: "docker:test:up"},
}

const nuxtDrizzleMigrationsDir = "server/database/migrations"

// maxDBNameLen ist das Postgres-Limit für Bezeichner.
const maxDBNameLen = 63

// containerStartTimeout begrenzt das Warten auf einen frisch gestarteten Container.
const containerStartTimeout = 60 * time.Second

type database struct {
	container string
	user      string
	name      string
}

func nuxtDrizzleSetup(e Env) error {
	if err := createDatabases(e, false); err != nil {
		return err
	}
	if err := Command(e, e.Worktree, "pnpm", "install", "--frozen-lockfile", "--prefer-offline"); err != nil {
		return err
	}
	return migrate(e)
}

// nuxtDrizzleResetDatabase löscht die Worktree-Datenbanken und migriert sie neu. Nach einem
// Rebase passen die angewendeten Migrationen sonst nicht mehr zu den Dateien.
func nuxtDrizzleResetDatabase(e Env) error {
	if err := createDatabases(e, true); err != nil {
		return err
	}
	return migrate(e)
}

func nuxtDrizzleCheck(e Env) error {
	for _, script := range []string{"lint", "typecheck", "test:run"} {
		if err := Command(e, e.Worktree, "pnpm", script); err != nil {
			return err
		}
	}
	return nil
}

// createDatabases schreibt die .env-Dateien und legt die Datenbanken an. Mit recreate löscht
// es bestehende Datenbanken vorher.
func createDatabases(e Env, recreate bool) error {
	for _, f := range nuxtDrizzleEnvFiles {
		db, err := prepareEnvFile(e, f)
		if err != nil {
			return err
		}
		if recreate {
			if err := dropDatabase(e, db); err != nil {
				return err
			}
		}
		if err := createDatabase(e, db); err != nil {
			return err
		}
	}
	return nil
}

func migrate(e Env) error {
	for _, script := range []string{"db:migrate", "test:db:push"} {
		if err := Command(e, e.Worktree, "pnpm", script); err != nil {
			return err
		}
	}
	return nil
}

// prepareEnvFile kopiert die Datei aus dem Hauptordner, hängt den Slug an den Datenbanknamen
// und sorgt dafür, dass der Container läuft.
func prepareEnvFile(e Env, f envFile) (database, error) {
	data, err := os.ReadFile(filepath.Join(e.ProjectDir, f.name))
	if err != nil {
		return database{}, fmt.Errorf("%s im Hauptordner: %w", f.name, err)
	}
	vals := parseEnv(string(data))
	base, port, user := vals["NUXT_DATABASE_NAME"], vals["NUXT_DATABASE_PORT"], vals["NUXT_DATABASE_USER"]
	if base == "" || port == "" || user == "" {
		return database{}, fmt.Errorf("%s: NUXT_DATABASE_NAME, NUXT_DATABASE_PORT und NUXT_DATABASE_USER müssen gesetzt sein", f.name)
	}
	name := base + "_" + e.Slug
	if len(name) > maxDBNameLen {
		return database{}, fmt.Errorf("Datenbankname %s ist länger als %d Zeichen", name, maxDBNameLen)
	}

	out := setEnv(string(data), "NUXT_DATABASE_NAME", name)
	if f.setPort {
		out = setEnv(out, "PORT", strconv.Itoa(e.Port))
	}
	if err := os.WriteFile(filepath.Join(e.Worktree, f.name), []byte(out), 0o600); err != nil {
		return database{}, err
	}
	e.Log.Printf("%s: NUXT_DATABASE_NAME=%s", f.name, name)

	hostPort, err := strconv.Atoi(port)
	if err != nil {
		return database{}, fmt.Errorf("%s: NUXT_DATABASE_PORT %q ist keine Zahl", f.name, port)
	}
	container, err := ensureContainer(e, hostPort, user, f.startScript)
	if err != nil {
		return database{}, err
	}
	return database{container: container, user: user, name: name}, nil
}

// ensureContainer findet den Postgres-Container über den Host-Port. Läuft er nicht, startet
// ensureContainer ihn über das Skript des Projekts und wartet, bis Postgres Verbindungen annimmt.
func ensureContainer(e Env, hostPort int, user, startScript string) (string, error) {
	c, ok, err := docker.ByHostPort(hostPort)
	if err != nil {
		return "", err
	}
	if !ok {
		e.Log.Printf("Kein Container auf Port %d, starte pnpm %s im Hauptordner", hostPort, startScript)
		if err := Command(e, e.ProjectDir, "pnpm", startScript); err != nil {
			return "", err
		}
	}
	deadline := time.Now().Add(containerStartTimeout)
	for {
		if c, ok, err = docker.ByHostPort(hostPort); err != nil {
			return "", err
		}
		if ok {
			if _, err := docker.Exec(c.Names, "pg_isready", "-q", "-U", user); err == nil {
				return c.Names, nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("Postgres auf Port %d ist nach %s nicht bereit", hostPort, containerStartTimeout)
		}
		time.Sleep(time.Second)
	}
}

func databaseExists(db database) (bool, error) {
	query := "SELECT 1 FROM pg_database WHERE datname = '" + strings.ReplaceAll(db.name, "'", "''") + "'"
	out, err := docker.Exec(db.container, "psql", "-U", db.user, "-d", "postgres", "-tAc", query)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}

// createDatabase lässt eine bestehende Datenbank stehen, damit ein wiederholtes Setup
// nach einem Fehler weiterläuft.
func createDatabase(e Env, db database) error {
	exists, err := databaseExists(db)
	if err != nil {
		return err
	}
	if exists {
		e.Log.Printf("Datenbank %s in %s existiert schon", db.name, db.container)
		return nil
	}
	if _, err := docker.Exec(db.container, "createdb", "-U", db.user, db.name); err != nil {
		return err
	}
	e.Log.Printf("Datenbank %s in %s angelegt", db.name, db.container)
	return nil
}

// dropDatabase trennt mit --force offene Verbindungen, etwa von einem vergessenen Dev-Server
// im Worktree.
func dropDatabase(e Env, db database) error {
	// Schutz der Haupt-Datenbanken: Gelöscht wird nur ein Name mit dem Slug des Worktrees.
	if e.Slug == "" || !strings.HasSuffix(db.name, "_"+e.Slug) {
		return fmt.Errorf("Datenbank %s gehört nicht zum Worktree, lösche sie nicht", db.name)
	}
	if _, err := docker.Exec(db.container, "dropdb", "--if-exists", "--force", "-U", db.user, db.name); err != nil {
		return err
	}
	e.Log.Printf("Datenbank %s in %s gelöscht", db.name, db.container)
	return nil
}

// nuxtDrizzleTeardown löscht nur Datenbanken mit dem Slug des Worktrees im Namen. Die
// Haupt-Datenbanken bleiben auch dann unberührt, wenn eine .env im Worktree fehlt oder
// verändert ist.
func nuxtDrizzleTeardown(e Env) error {
	var errs []error
	for _, f := range nuxtDrizzleEnvFiles {
		data, err := os.ReadFile(filepath.Join(e.ProjectDir, f.name))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s im Hauptordner: %w", f.name, err))
			continue
		}
		vals := parseEnv(string(data))
		port, err := strconv.Atoi(vals["NUXT_DATABASE_PORT"])
		if err != nil || vals["NUXT_DATABASE_NAME"] == "" || e.Slug == "" {
			errs = append(errs, fmt.Errorf("%s: Datenbank nicht bestimmbar", f.name))
			continue
		}
		db := database{user: vals["NUXT_DATABASE_USER"], name: vals["NUXT_DATABASE_NAME"] + "_" + e.Slug}
		c, ok, err := docker.ByHostPort(port)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !ok {
			e.Log.Printf("Kein Container auf Port %d, Datenbank %s bleibt bestehen", port, db.name)
			continue
		}
		db.container = c.Names
		if err := dropDatabase(e, db); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// mcpFiles schreibt der Dev-Server mit seinem Port neu. Geänderte Dateien lassen den Worktree
// unsauber aussehen, und git rebase bricht ab.
var mcpFiles = []string{".mcp.json", ".vscode/mcp.json"}

// resetMCPFiles setzt die getrackten MCP-Dateien auf den Stand des Commits zurück.
func resetMCPFiles(e Env) error {
	for _, f := range mcpFiles {
		if !git.IsTracked(e.Worktree, f) {
			continue
		}
		if _, err := git.Run(e.Worktree, "diff", "--quiet", "HEAD", "--", f); err == nil {
			continue
		}
		if _, err := git.Run(e.Worktree, "checkout", "HEAD", "--", f); err != nil {
			return err
		}
		e.Log.Printf("%s auf den Stand von HEAD zurückgesetzt", f)
	}
	return nil
}
