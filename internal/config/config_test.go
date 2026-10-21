package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIDs(t *testing.T) {
	ids := IDs([]string{"/a/shop", "/b/blog", "/c/shop"})
	if ids[1] != "blog" {
		t.Errorf("unique name: %q", ids[1])
	}
	for _, id := range []string{ids[0], ids[2]} {
		suffix, ok := strings.CutPrefix(id, "shop-")
		if !ok || len(suffix) != 6 {
			t.Errorf("shared name: %q", id)
		}
	}
	if ids[0] == ids[2] {
		t.Errorf("shared names got the same ID: %q", ids[0])
	}
	// The ID of a project does not depend on the order of the list.
	rev := IDs([]string{"/c/shop", "/b/blog", "/a/shop"})
	if rev[0] != ids[2] || rev[2] != ids[0] || rev[1] != "blog" {
		t.Errorf("order changes IDs: %v vs %v", ids, rev)
	}
	if got := IDs(nil); len(got) != 0 {
		t.Errorf("empty list: %v", got)
	}
}

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for in, want := range map[string]string{
		"~":          home,
		"~/code/x/":  filepath.Join(home, "code", "x"),
		"/a/../b//c": "/b/c",
	} {
		if got, err := ExpandPath(in); err != nil || got != want {
			t.Errorf("ExpandPath(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	wd, _ := os.Getwd()
	if got, _ := ExpandPath("rel/dir"); got != filepath.Join(wd, "rel", "dir") {
		t.Errorf("relative: %q", got)
	}
	// "~user" is no home folder of lazydev's.
	if got, _ := ExpandPath("~other"); got != filepath.Join(wd, "~other") {
		t.Errorf("~other: %q", got)
	}
}

func TestDisplayPath(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	for in, want := range map[string]string{
		"/home/u":       "~",
		"/home/u/code":  "~/code",
		"/home/user2/x": "/home/user2/x",
		"/srv":          "/srv",
	} {
		if got := DisplayPath(in); got != want {
			t.Errorf("DisplayPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompletePath(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"project-a", "project-b", "single", ".hidden", "other"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	os.WriteFile(filepath.Join(root, "sinfile"), nil, 0o644)
	os.WriteFile(filepath.Join(root, "projfile"), nil, 0o644)

	for in, want := range map[string]string{
		root + "/sin":       root + "/single/",
		root + "/pro":       root + "/project-",
		root + "/proj":      root + "/project-",
		root + "/zzz":       root + "/zzz",
		root + "/.h":        root + "/.hidden/",
		root + "/":          root + "/",
		root + "/other":     root + "/other/",
		"nodir":             "nodir",
		root + "/missing/x": root + "/missing/x",
	} {
		if got := CompletePath(in); got != want {
			t.Errorf("CompletePath(%q) = %q, want %q", in, got, want)
		}
	}
	// Without a dot, hidden folders do not count: "" matches all visible ones, no common prefix.
	if got := CompletePath(root + "/"); strings.Contains(got, "hidden") {
		t.Errorf("hidden folder completed: %q", got)
	}
}

func TestCompletePathHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, "workspace"), 0o755)
	if got := CompletePath("~/work"); got != "~/workspace/" {
		t.Errorf("CompletePath(~/work) = %q", got)
	}
}

func TestCompletePathSymlink(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "target"), 0o755)
	os.Symlink(filepath.Join(root, "target"), filepath.Join(root, "linked"))
	if got := CompletePath(root + "/lin"); got != root+"/linked/" {
		t.Errorf("linked folder: %q", got)
	}
}

func TestProjects(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got, err := LoadProjects(); got != nil || err != nil {
		t.Fatalf("missing file: %v, %v", got, err)
	}
	paths := []string{"/a/shop", "/b/blog"}
	if err := SaveProjects(paths); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProjects()
	if err != nil || !slices.Equal(got, paths) {
		t.Errorf("roundtrip: %v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(Dir(), "projects.json")); err != nil {
		t.Error(err)
	}
	os.WriteFile(projectsPath(), []byte("{broken"), 0o644)
	if _, err := LoadProjects(); err == nil {
		t.Error("broken file gives no error")
	}
}

func TestSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if s := LoadSettings(); s != (Settings{}) {
		t.Errorf("missing file: %+v", s)
	}
	if err := SaveSettings(Settings{Theme: "Tokyo Night"}); err != nil {
		t.Fatal(err)
	}
	if s := LoadSettings(); s.Theme != "Tokyo Night" {
		t.Errorf("roundtrip: %+v", s)
	}
	os.WriteFile(settingsPath(), []byte("not json"), 0o644)
	if s := LoadSettings(); s != (Settings{}) {
		t.Errorf("broken file: %+v", s)
	}
}

func TestCompletePathMultibytePrefix(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "ä"), 0o755)
	os.MkdirAll(filepath.Join(root, "ö"), 0o755)
	// "ä" and "ö" share their first UTF-8 byte, but no character.
	if got := CompletePath(root + "/"); got != root+"/" {
		t.Errorf("CompletePath returned half a UTF-8 character: %q", got)
	}
}
