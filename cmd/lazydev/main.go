// lazydev startet ohne Argumente die TUI. Die Unterbefehle steuern den Lauf eines PRDs.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazydev/internal/chatrun"
	"lazydev/internal/config"
	"lazydev/internal/runner"
	"lazydev/internal/state"
	"lazydev/internal/tui"
	"lazydev/internal/worktree"
)

const usage = `Aufruf:
  lazydev                                          TUI starten
  lazydev start <projekt-id> <prd-name>            Runner im Hintergrund starten, mit maxParallel
  lazydev run <projekt-id> <prd-name>              Runner im Vordergrund: Setup, Umsetzung, Review
  lazydev merge <projekt-id> <prd-name>            Merge freigeben, wenn autoMerge aus ist
  lazydev stop <projekt-id> <prd-name>             Runner stoppen, der Worktree bleibt
  lazydev teardown [--yes] <projekt-id> <prd-name> Lauf verwerfen: Datenbanken, Worktree, Branch
  lazydev chat <projekt-id> <chat-id>              Eine Runde eines PRD-Chats ausführen, die TUI startet sie
  lazydev trust <projekt-id>                       Hooks und Freigaben aus .lazydev/config.yaml prüfen und bestätigen

Die Projekt-ID ist der Ordnername des Projekts. Der PRD-Name ist der Dateiname ohne .md.
Nach einem Fehler oder Abbruch macht start oder run beim gescheiterten Schritt weiter.
Logs: %s

Konfiguration: %s
`

func main() {
	cfgPath := config.DefaultPath()
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			fmt.Printf(usage, state.Dir(), cfgPath)
			return
		case "start", "run", "merge", "stop", "teardown", "chat", "trust":
		default:
			fmt.Fprintf(os.Stderr, "Unbekannter Befehl: %s\n\n"+usage, args[0], state.Dir(), cfgPath)
			os.Exit(2)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fail(err)
	}
	if len(args) == 0 {
		p := tea.NewProgram(tui.New(cfgPath, cfg).WithWatch(), tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fail(err)
		}
		return
	}

	cmd, rest := args[0], args[1:]
	if cmd == "trust" {
		if len(rest) != 1 {
			fmt.Fprintf(os.Stderr, usage, state.Dir(), cfgPath)
			os.Exit(2)
		}
		if err := trust(cfg, rest[0]); err != nil {
			fail(err)
		}
		return
	}
	yes := false
	if cmd == "teardown" && len(rest) > 0 && rest[0] == "--yes" {
		yes, rest = true, rest[1:]
	}
	if len(rest) != 2 {
		fmt.Fprintf(os.Stderr, usage, state.Dir(), cfgPath)
		os.Exit(2)
	}
	target, err := runner.FindProject(cfg, rest[0])
	if err != nil {
		fail(err)
	}
	prdName := rest[1]

	switch cmd {
	case "start", "merge":
		start := runner.Start
		if cmd == "merge" {
			start = runner.Merge
		}
		var started bool
		if started, err = start(cfg, target, prdName); err == nil {
			if started {
				fmt.Printf("Runner gestartet. Log: %s\n", state.RunnerLogPath(target.ID, prdName))
			} else {
				fmt.Printf("maxParallel (%d) erreicht. %s wartet und startet, sobald ein Runner endet.\n", cfg.MaxParallel, prdName)
			}
		}
	case "run":
		err = runner.Run(cfg, target, prdName, os.Stdout)
	case "stop":
		if err = runner.Stop(target, prdName); err == nil {
			fmt.Println("SIGTERM gesendet.")
		}
	case "teardown":
		if !yes && !confirm(fmt.Sprintf("Datenbanken, Worktree und Branch %s von %s löschen? Ungemergte Commits gehen verloren. (j/n) ",
			worktree.Branch(prdName), prdName)) {
			fmt.Println("Abgebrochen.")
			return
		}
		err = runner.Teardown(target, prdName, os.Stdout)
	case "chat":
		err = chatrun.Round(cfg, target.ID, rest[1])
	}
	if err != nil {
		fail(err)
	}
}

// trust zeigt Hooks und Freigaben aus .lazydev/config.yaml und speichert ihren Hash nach "j".
func trust(cfg *config.Config, projectID string) error {
	target, err := runner.FindProject(cfg, projectID)
	if err != nil {
		return err
	}
	repo, exists, err := config.LoadRepo(target.Dir())
	if err != nil {
		return err
	}
	hash := repo.TrustHash()
	if !exists || hash == "" {
		fmt.Printf("%s setzt keine Hooks oder Freigaben in %s/config.yaml. Es gibt nichts zu bestätigen.\n", projectID, config.RepoDir)
		return nil
	}
	fmt.Printf("%s/config.yaml von %s setzt Hooks oder Freigaben:\n\n%s\n", config.RepoDir, projectID, repo.TrustSections())
	if !confirm("Erlauben? (j/n) ") {
		fmt.Println("Abgebrochen.")
		return nil
	}
	if err := state.Trust(target.ID, hash); err != nil {
		return err
	}
	fmt.Println("Bestätigt.")
	return nil
}

func confirm(question string) bool {
	fmt.Print(question)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	a := strings.ToLower(strings.TrimSpace(answer))
	return a == "j" || a == "y"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lazydev:", err)
	os.Exit(1)
}
