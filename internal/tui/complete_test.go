package tui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCompletePath(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"nuxt-drizzle", "nuxt-ui", "other", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "nuxt-file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "other"), filepath.Join(root, "otherlink")); err != nil {
		t.Fatal(err)
	}

	got, cands := completePath(root + "/nu")
	if got != root+"/nuxt-" || !slices.Equal(cands, []string{"nuxt-drizzle", "nuxt-ui"}) {
		t.Errorf("mehrere Treffer: %q %v", got, cands)
	}
	if got, cands := completePath(root + "/nuxt-d"); got != root+"/nuxt-drizzle/" || cands != nil {
		t.Errorf("ein Treffer: %q %v", got, cands)
	}
	if got, _ := completePath(root + "/x"); got != root+"/x" {
		t.Errorf("kein Treffer: %q", got)
	}
	if _, cands := completePath(root + "/"); slices.Contains(cands, ".hidden") || !slices.Contains(cands, "otherlink") {
		t.Errorf("versteckte Ordner oder Symlinks falsch: %v", cands)
	}
	if got, _ := completePath(root + "/.h"); got != root+"/.hidden/" {
		t.Errorf("versteckter Ordner mit Punkt: %q", got)
	}
}

func TestCommonPrefixKeepsRunes(t *testing.T) {
	if got := commonPrefix([]string{"bär", "bör"}); got != "b" {
		t.Errorf("commonPrefix = %q", got)
	}
}

func TestScrollWindow(t *testing.T) {
	if s, e := scrollWindow(0, 3, 10); s != 0 || e != 3 {
		t.Errorf("kurze Liste: %d %d", s, e)
	}
	if s, e := scrollWindow(19, 20, 5); s != 15 || e != 20 {
		t.Errorf("Ende: %d %d", s, e)
	}
	if s, e := scrollWindow(10, 20, 5); s > 10 || e <= 10 {
		t.Errorf("Cursor nicht sichtbar: %d %d", s, e)
	}
}
