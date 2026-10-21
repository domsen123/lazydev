package repo

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// settingsDir creates a project with config.yaml content.
func settingsDir(t *testing.T, content string) string {
	t.Helper()
	setupEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/config.yaml", content)
	return dir
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name, in, base string
		knowledge      map[string][]string
		want           string
		changed        bool
	}{
		{
			name: "empty file, no change",
			in:   "",
			base: "main",
			want: "",
		},
		{
			name: "empty base means main",
			in:   "",
			base: "",
			want: "",
		},
		{
			name:    "empty file, base branch",
			in:      "",
			base:    "develop",
			want:    "baseBranch: develop\n",
			changed: true,
		},
		{
			name:      "empty file, knowledge in key order",
			in:        "",
			base:      "main",
			knowledge: map[string][]string{"merge": {"c.md"}, "implement": {"b.md", "a.md"}, "all": {"CLAUDE.md"}, "review": nil},
			want:      "knowledge:\n  all:\n    - CLAUDE.md\n  implement:\n    - b.md\n    - a.md\n  merge:\n    - c.md\n",
			changed:   true,
		},
		{
			name:    "main drops baseBranch and keeps its comment",
			in:      "# base\nbaseBranch: develop\nhooks:\n  dev: ./d.sh\n",
			base:    "main",
			want:    "# base\nhooks:\n  dev: ./d.sh\n",
			changed: true,
		},
		{
			name:    "replace baseBranch in place",
			in:      "# top\n\npreset: nuxt-drizzle # old\nbaseBranch: develop # target\nallowedTools:\n  - Bash(x)\n",
			base:    "next",
			want:    "# top\n\npreset: nuxt-drizzle # old\nbaseBranch: next # target\nallowedTools:\n  - Bash(x)\n",
			changed: true,
		},
		{
			name:    "new baseBranch goes first below the head comment",
			in:      "# top\n\npreset: nuxt-drizzle\n",
			base:    "develop",
			want:    "# top\n\nbaseBranch: develop\npreset: nuxt-drizzle\n",
			changed: true,
		},
		{
			name:      "same values, no change",
			in:        "# keep\nbaseBranch:   develop\nknowledge:\n  implement: [a.md]\n  all: [CLAUDE.md]\n",
			base:      "develop",
			knowledge: map[string][]string{"all": {"CLAUDE.md"}, "implement": {"a.md"}},
			want:      "# keep\nbaseBranch:   develop\nknowledge:\n  implement: [a.md]\n  all: [CLAUDE.md]\n",
		},
		{
			name:      "empty lists count as missing",
			in:        "knowledge:\n  all: []\n",
			base:      "main",
			knowledge: map[string][]string{"review": {}},
			want:      "knowledge:\n  all: []\n",
		},
		{
			name:      "replace knowledge, keep its comment and position",
			in:        "baseBranch: develop\n# read first\nknowledge:\n  all: [old.md]\nhooks:\n  dev: ./d.sh\n",
			base:      "develop",
			knowledge: map[string][]string{"all": {"new.md"}},
			want:      "baseBranch: develop\n# read first\nknowledge:\n  all:\n    - new.md\nhooks:\n  dev: ./d.sh\n",
			changed:   true,
		},
		{
			name:      "new knowledge goes to the end",
			in:        "hooks:\n  dev: ./d.sh\n",
			base:      "main",
			knowledge: map[string][]string{"review": {"r.md"}},
			want:      "hooks:\n  dev: ./d.sh\nknowledge:\n  review:\n    - r.md\n",
			changed:   true,
		},
		{
			name:    "removing knowledge deletes the key",
			in:      "baseBranch: develop\nknowledge:\n  all: [CLAUDE.md]\nhooks:\n  dev: ./d.sh\n",
			base:    "develop",
			want:    "baseBranch: develop\nhooks:\n  dev: ./d.sh\n",
			changed: true,
		},
		{
			name:    "removing everything gives an empty file",
			in:      "baseBranch: develop\nknowledge:\n  all: [CLAUDE.md]\n",
			base:    "main",
			want:    "",
			changed: true,
		},
		{
			// The comment stays, the empty mapping then encodes as "{}".
			name:    "removing everything keeps a comment",
			in:      "# lazydev\nbaseBranch: develop\n",
			base:    "main",
			want:    "{}\n# lazydev\n",
			changed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := settingsDir(t, tt.in)
			out, changed, err := Settings(dir, tt.base, tt.knowledge)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tt.want || changed != tt.changed {
				t.Errorf("Settings =\n%q, %v\nwant\n%q, %v", out, changed, tt.want, tt.changed)
			}
			// Settings only returns the content, the file stays.
			if got := readFile(t, ConfigPath(dir)); got != tt.in {
				t.Errorf("file changed: %q", got)
			}
			// The result reads back with the new values.
			var c Config
			if err := yaml.Unmarshal(out, &c); err != nil {
				t.Fatalf("result does not parse: %v", err)
			}
			wantBase := tt.base
			if wantBase == "" {
				wantBase = DefaultBaseBranch
			}
			if c.Base() != wantBase || !equalKnowledge(compactKnowledge(c.Knowledge), compactKnowledge(tt.knowledge)) {
				t.Errorf("result reads as %+v", c)
			}
		})
	}
}

func TestSettingsKeepsUnknownFieldsAndOrder(t *testing.T) {
	in := `# Settings for lazydev
preset: nuxt-drizzle
baseBranch: develop
allowedTools:
  - "Bash(pnpm test:*)"
# knowledge per step
knowledge:
  all: [CLAUDE.md]
hooks:
  # runs first
  setup: ./scripts/setup.sh
extra:
  nested: true
`
	dir := settingsDir(t, in)
	out, changed, err := Settings(dir, "next", map[string][]string{"all": {"CLAUDE.md"}, "implement": {"docs/db.md"}})
	if err != nil || !changed {
		t.Fatalf("Settings = %v, %v", changed, err)
	}
	got := string(out)
	for _, want := range []string{"# Settings for lazydev\n", "preset: nuxt-drizzle\n", "baseBranch: next\n", `- "Bash(pnpm test:*)"`,
		"# knowledge per step\nknowledge:\n", "  implement:\n    - docs/db.md\n", "  # runs first\n  setup: ./scripts/setup.sh\n", "extra:\n  nested: true\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	order := []string{"preset:", "baseBranch:", "allowedTools:", "knowledge:", "hooks:", "extra:"}
	last := -1
	for _, key := range order {
		i := strings.Index(got, "\n"+key)
		if i < last {
			t.Errorf("%s out of order in\n%s", key, got)
		}
		last = i
	}
}

func TestSettingsErrors(t *testing.T) {
	setupEnv(t)
	if _, _, err := Settings(t.TempDir(), "main", nil); err == nil {
		t.Error("no error without config.yaml")
	}
	for _, in := range []string{"baseBranch: [x\n", "- a\n- b\n", "plain\n", "knowledge: [a.md]\n"} {
		dir := settingsDir(t, in)
		if _, _, err := Settings(dir, "develop", map[string][]string{"all": {"a.md"}}); err == nil || !strings.Contains(err.Error(), "config.yaml") {
			t.Errorf("Settings(%q) error = %v", in, err)
		}
	}
}
