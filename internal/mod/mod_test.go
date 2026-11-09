package mod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallUpdateUninstall(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if changed, err := Update(); err != nil || changed || Installed() {
		t.Fatalf("Update without mod = %v, %v", changed, err)
	}
	if changed, err := Install(); err != nil || !changed {
		t.Fatalf("Install = %v, %v", changed, err)
	}
	for _, f := range []string{".claude-plugin/plugin.json", "hooks/hooks.json", "hooks/register.tsx", "types/index.d.ts", "README.md"} {
		if _, err := os.Stat(filepath.Join(Dir(), f)); err != nil {
			t.Errorf("%s missing: %v", f, err)
		}
	}
	if !strings.HasSuffix(Dir(), filepath.Join("skills", "lazydev-runs")) {
		t.Errorf("Dir = %s", Dir())
	}
	stale := filepath.Join(Dir(), "hooks", "old.tsx")
	os.WriteFile(stale, []byte("x"), 0o644)
	if changed, err := Update(); err != nil || !changed {
		t.Fatalf("Update = %v, %v", changed, err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("Update kept a file the binary does not have")
	}
	if changed, _ := Install(); changed {
		t.Error("second Install changed something")
	}
	if err := Uninstall(); err != nil || Installed() {
		t.Errorf("Uninstall = %v, installed %v", err, Installed())
	}
}

// TestManifestNamesTheMod checks that plugin.json and the module agree on the name, which the
// state contract and the pane id use too.
func TestManifestNamesTheMod(t *testing.T) {
	manifest, _ := files.ReadFile("files/.claude-plugin/plugin.json")
	module, _ := files.ReadFile("files/hooks/register.tsx")
	if !strings.Contains(string(manifest), `"name": "`+Name+`"`) || !strings.Contains(string(module), "plugin: '"+Name+"'") {
		t.Errorf("name %s not in manifest or module", Name)
	}
}
