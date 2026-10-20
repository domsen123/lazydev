package worktree

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	tests := map[string]string{
		"mobile-bearer-token-auth": "mobile_bearer_token_auth",
		"Avatar Upload--v2":        "avatar_upload_v2",
		"-x-":                      "x",
		strings.Repeat("ab-", 20):  "ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_ab_a",
	}
	for in, want := range tests {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, erwartet %q", in, got, want)
		}
		if len(Slug(in)) > MaxSlugLen {
			t.Errorf("Slug(%q) zu lang", in)
		}
	}
	if got := Slug(strings.Repeat("a", 39) + "-b"); strings.HasSuffix(got, "_") {
		t.Errorf("Slug endet mit Unterstrich: %q", got)
	}
}

func TestAllocatePort(t *testing.T) {
	// Einen freien Port suchen und belegen, damit AllocatePort ihn überspringen muss.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	busy := l.Addr().(*net.TCPAddr).Port

	if PortFree(busy) {
		t.Errorf("Port %d gilt als frei, obwohl belegt", busy)
	}
	got, err := AllocatePort(busy, map[int]bool{busy + 1: true})
	if err != nil {
		t.Fatal(err)
	}
	if got == busy || got == busy+1 {
		t.Errorf("AllocatePort lieferte belegten Port %d", got)
	}
}

func TestPortFreeIPv6(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("kein IPv6:", err)
	}
	defer l.Close()
	if PortFree(l.Addr().(*net.TCPAddr).Port) {
		t.Error("Port nur auf [::1] belegt, gilt aber als frei")
	}
}

func TestCreateRemove(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")

	wt := filepath.Join(t.TempDir(), "proj", "feature")
	if err := Create(repo, wt, "feat/feature"); err != nil {
		t.Fatal(err)
	}
	if err := Create(repo, wt, "feat/feature"); err == nil {
		t.Error("zweites Create ohne Fehler")
	}
	if out := runGit(t, wt, "branch", "--show-current"); out != "feat/feature" {
		t.Errorf("Branch im Worktree = %q", out)
	}

	os.WriteFile(filepath.Join(wt, "dirty.txt"), []byte("x"), 0o644)
	if err := Remove(repo, wt, "feat/feature", false); err == nil {
		t.Error("Remove ohne force entfernte einen Worktree mit Änderungen")
	}
	if err := Remove(repo, wt, "feat/feature", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); err == nil {
		t.Error("Worktree-Ordner existiert noch")
	}
	if out := runGit(t, repo, "branch", "--list", "feat/feature"); out != "" {
		t.Errorf("Branch existiert noch: %q", out)
	}
}

func TestPathUsesXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data")
	if got := Path("proj", "x"); got != "/data/lazydev/worktrees/proj/x" {
		t.Errorf("Path = %q", got)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
