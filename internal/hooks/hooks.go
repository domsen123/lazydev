// Package hooks führt Setup und Teardown eines Worktrees aus: eingebaute Presets oder
// Skripte aus der Projekt-Konfiguration.
package hooks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"lazydev/internal/config"
	"lazydev/internal/runlog"
)

// Env enthält die Werte eines Worktrees. Skript-Hooks bekommen sie als LAZYDEV_*-Variablen.
type Env struct {
	ProjectDir string
	Worktree   string
	Branch     string
	Port       int
	Slug       string
	Log        *runlog.Logger
}

func (e Env) Vars() []string {
	return []string{
		"LAZYDEV_PROJECT=" + e.ProjectDir,
		"LAZYDEV_WORKTREE=" + e.Worktree,
		"LAZYDEV_BRANCH=" + e.Branch,
		"LAZYDEV_PORT=" + strconv.Itoa(e.Port),
		"LAZYDEV_SLUG=" + e.Slug,
	}
}

type Hook func(Env) error

type Hooks struct {
	Setup    Hook
	Teardown Hook
	// Clean setzt erzeugte Dateien wie .mcp.json zurück, vor jedem Claude-Schritt, nach dem
	// Review und vor dem Rebase.
	Clean Hook
	// Check führt die Prüfbefehle des Projekts im Worktree aus.
	Check Hook
	// ResetDatabase legt die Worktree-Datenbanken neu an, wenn ein Rebase Migrationen aus main
	// geholt hat.
	ResetDatabase Hook
	// MigrationsDir ist der Ordner der Migrationen relativ zum Projekt. Leer heißt: keine.
	MigrationsDir string
	// Dev startet den Dev-Server im Worktree. Leer heißt: kein Dev-Server.
	Dev []string
	// IgnoreDirty listet Dateien, die im Hauptordner geändert sein dürfen, z. B. vom
	// Dev-Server neu geschriebene. Der Fast-Forward bricht ab, falls der Merge sie berührt.
	IgnoreDirty []string
}

func noop(Env) error { return nil }

// For liefert die Hooks eines Projekts. Skripte aus hooks.setup, hooks.teardown, hooks.check
// und hooks.dev ersetzen den jeweiligen Hook des Presets. Ohne Preset und Skript tun die Hooks nichts.
func For(p config.Project) Hooks {
	h := Hooks{Setup: noop, Teardown: noop, Clean: noop, Check: noop, ResetDatabase: noop}
	if p.Preset == config.PresetNuxtDrizzle {
		h = Hooks{
			Setup:         nuxtDrizzleSetup,
			Teardown:      nuxtDrizzleTeardown,
			Clean:         resetMCPFiles,
			Check:         nuxtDrizzleCheck,
			ResetDatabase: nuxtDrizzleResetDatabase,
			MigrationsDir: nuxtDrizzleMigrationsDir,
			Dev:           []string{"pnpm", "dev"},
			IgnoreDirty:   mcpFiles,
		}
	}
	if p.Hooks.Setup != "" {
		h.Setup = script(p.Hooks.Setup)
	}
	if p.Hooks.Teardown != "" {
		h.Teardown = script(p.Hooks.Teardown)
	}
	if p.Hooks.Check != "" {
		h.Check = script(p.Hooks.Check)
	}
	if p.Hooks.Dev != "" {
		h.Dev = []string{scriptPath(p, p.Hooks.Dev)}
	}
	return h
}

// scriptPath löst einen relativen Skriptpfad ab dem Hauptordner auf.
func scriptPath(p config.Project, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.AbsPath(), path)
}

// script führt ein Skript im Worktree aus. Relative Pfade gelten ab dem Hauptordner des Projekts.
func script(path string) Hook {
	return func(e Env) error {
		p := path
		if !filepath.IsAbs(p) {
			p = filepath.Join(e.ProjectDir, p)
		}
		return Command(e, e.Worktree, p)
	}
}

// Command führt einen Befehl aus und schreibt seine Ausgabe zeilenweise ins Log.
// stdin bleibt leer, damit kein Befehl auf Eingabe wartet.
func Command(e Env, dir, name string, args ...string) error {
	e.Log.Printf("$ %s", strings.Join(append([]string{name}, args...), " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), e.Vars()...)
	out := e.Log.Writer()
	defer out.Close()
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return fmt.Errorf("%s endete mit Exit-Code %d", name, exit.ExitCode())
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
