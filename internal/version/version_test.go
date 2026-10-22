package version

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRelease(t *testing.T) {
	for _, tc := range []struct {
		v      string
		ok     bool
		mj, mn int
		p      int
	}{
		{"v1.2.3", true, 1, 2, 3},
		{"v0.10.0", true, 0, 10, 0},
		{"1.2.3", false, 0, 0, 0},
		{"v1.2", false, 0, 0, 0},
		{"v1.2.3-rc1", false, 0, 0, 0},
		{"v0.1.0-3-gabc", false, 0, 0, 0},
		{"v1.2.3.4", false, 0, 0, 0},
		{"v1..3", false, 0, 0, 0},
		{"v+1.2.3", false, 0, 0, 0},
		{"dev", false, 0, 0, 0},
		{"", false, 0, 0, 0},
	} {
		mj, mn, p, ok := Release(tc.v)
		if ok != tc.ok || mj != tc.mj || mn != tc.mn || p != tc.p {
			t.Errorf("Release(%q) = %d, %d, %d, %v", tc.v, mj, mn, p, ok)
		}
	}
}

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		want            bool
	}{
		{"v0.10.0", "v0.9.0", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.1.1", "v0.1.0", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.9.0", "v0.10.0", false},
		{"v0.2.0", "dev", false},
		{"v0.2.0", "v0.1.0-3-gabc", false},
		{"v0.2.0-rc1", "v0.1.0", false},
		{"", "v0.1.0", false},
	} {
		if got := Newer(tc.latest, tc.current); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v", tc.latest, tc.current, got)
		}
	}
}

// fakeGH sets GH to a script with body for the test.
func fakeGH(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := GH
	GH = path
	t.Cleanup(func() { GH = old })
}

func TestLatest(t *testing.T) {
	fakeGH(t, `[ "$GH_PROMPT_DISABLED" = 1 ] || exit 3
[ "$*" = "release view --repo domsen123/lazydev --json tagName --jq .tagName" ] || exit 4
echo " v0.2.0 "`)
	got, err := Latest(context.Background())
	if err != nil || got != "v0.2.0" {
		t.Errorf("Latest = %q, %v", got, err)
	}
}

func TestLatestError(t *testing.T) {
	fakeGH(t, "echo 'not logged in' >&2\nexit 1")
	if got, err := Latest(context.Background()); err == nil {
		t.Errorf("Latest = %q without error", got)
	}
}
