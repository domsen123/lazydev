// Package worktree legt Git-Worktrees für PRDs an und entfernt sie. Es vergibt Port und Slug.
package worktree

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"lazydev/internal/git"
	"lazydev/internal/project"
)

// MaxSlugLen hält Datenbanknamen wie "<name>_<slug>" unter dem Postgres-Limit von 63 Byte.
const MaxSlugLen = 40

// portSearchRange begrenzt die Suche nach einem freien Port ab portBase.
const portSearchRange = 1000

// Root beachtet XDG_DATA_HOME.
func Root() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "lazydev", "worktrees")
}

func Path(projectID, prdName string) string {
	return filepath.Join(Root(), projectID, prdName)
}

func Branch(prdName string) string {
	return project.FeatureBranchPrefix + prdName
}

// Slug macht aus "mobile-bearer-token-auth" den Datenbank-Zusatz "mobile_bearer_token_auth".
func Slug(prdName string) string {
	var b strings.Builder
	underscore := false
	for _, r := range strings.ToLower(prdName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			underscore = false
		} else if !underscore && b.Len() > 0 {
			b.WriteByte('_')
			underscore = true
		}
	}
	s := b.String()
	if len(s) > MaxSlugLen {
		s = s[:MaxSlugLen]
	}
	return strings.Trim(s, "_")
}

// AllocatePort liefert den ersten Port ab base, den weder ein anderer Lauf belegt noch ein
// Prozess auf IPv4 oder IPv6 nutzt. Nuxt lauscht nur auf [::1], deshalb prüft
// AllocatePort beide Adressfamilien.
func AllocatePort(base int, used map[int]bool) (int, error) {
	for p := base; p < base+portSearchRange; p++ {
		if !used[p] && PortFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("kein freier Port zwischen %d und %d", base, base+portSearchRange-1)
}

// PortFree meldet, ob sich der Port auf 127.0.0.1 und [::1] binden lässt.
func PortFree(port int) bool {
	for _, host := range []string{"127.0.0.1", "::1"} {
		l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			// Ohne IPv6 auf dem Rechner kann dort auch niemand lauschen.
			if host == "::1" && isAddrUnavailable(err) {
				continue
			}
			return false
		}
		l.Close()
	}
	return true
}

func isAddrUnavailable(err error) bool {
	return strings.Contains(err.Error(), "cannot assign requested address") ||
		strings.Contains(err.Error(), "address family not supported")
}

// Create legt den Worktree mit neuem Branch von main an.
func Create(mainDir, path, branch string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("Worktree-Ordner existiert schon: %s", path)
	}
	if git.BranchExists(mainDir, branch) {
		return fmt.Errorf("Branch %s existiert schon", branch)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err := git.Run(mainDir, "worktree", "add", "-b", branch, path, project.MainBranch)
	return err
}

// Remove entfernt Worktree und Branch. force verwirft auch ungemergte Commits und lokale
// Änderungen. Ohne force entfernt Remove nur einen sauberen Worktree und einen gemergten Branch.
func Remove(mainDir, path, branch string, force bool) error {
	var errs []error
	if _, err := os.Stat(path); err == nil {
		args := []string{"worktree", "remove", path}
		if force {
			args = []string{"worktree", "remove", "--force", path}
		}
		if _, err := git.Run(mainDir, args...); err != nil {
			errs = append(errs, err)
		}
	}
	// Ein von Hand gelöschter Ordner hinterlässt Verwaltungsdaten in .git/worktrees.
	if _, err := git.Run(mainDir, "worktree", "prune"); err != nil {
		errs = append(errs, err)
	}
	if git.BranchExists(mainDir, branch) {
		flag := "-d"
		if force {
			flag = "-D"
		}
		if _, err := git.Run(mainDir, "branch", flag, branch); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
