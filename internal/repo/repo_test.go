package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lazydev/internal/state"
)

// setupEnv isolates state and git config.
func setupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// initRepo creates a git repository on branch without commits.
func initRepo(t *testing.T, branch string) string {
	t.Helper()
	setupEnv(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", branch)
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
}

func workflowPrompt(t *testing.T, step string) string {
	t.Helper()
	data, err := simpleWorkflow.ReadFile("workflows/simple/" + step + ".md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWorkflowHasAllSteps(t *testing.T) {
	for _, step := range Steps {
		if p := workflowPrompt(t, step); strings.TrimSpace(p) == "" {
			t.Errorf("workflow prompt %s is empty", step)
		}
	}
	if !slices.Equal(KnowledgeKeys, []string{"all", "create", "implement", "review", "merge"}) {
		t.Errorf("KnowledgeKeys = %v", KnowledgeKeys)
	}
}

func TestInitMain(t *testing.T) {
	dir := initRepo(t, "main")
	commitAll(t, dir, "init")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, ConfigPath(dir)); got != "" {
		t.Errorf("config.yaml = %q, want empty", got)
	}
	for _, step := range Steps {
		if got := readFile(t, PromptPath(dir, step)); got != workflowPrompt(t, step) {
			t.Errorf("%s.md differs from the workflow", step)
		}
	}
	info := Read(dir, "p")
	if !info.Initialized || info.Err != nil || info.Config.Base() != "main" || info.Trust != TrustNone {
		t.Errorf("Read after Init = %+v", info)
	}
}

func TestInitOtherBaseBranch(t *testing.T) {
	dir := initRepo(t, "develop")
	commitAll(t, dir, "init")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, ConfigPath(dir)); got != "baseBranch: develop\n" {
		t.Errorf("config.yaml = %q", got)
	}
	if info := Read(dir, "p"); info.Config.Base() != "develop" {
		t.Errorf("Base = %q", info.Config.Base())
	}
}

func TestInitMasterWinsOverCurrent(t *testing.T) {
	dir := initRepo(t, "master")
	commitAll(t, dir, "init")
	gitRun(t, dir, "checkout", "-q", "-b", "topic")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, ConfigPath(dir)); got != "baseBranch: master\n" {
		t.Errorf("config.yaml = %q", got)
	}
}

func TestInitWithoutGit(t *testing.T) {
	setupEnv(t)
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, ConfigPath(dir)); got != "" {
		t.Errorf("config.yaml = %q", got)
	}
	info := Read(dir, "p")
	if !info.Initialized || !slices.Equal(info.Prompts, []bool{true, true, true, true}) {
		t.Errorf("Read = %+v", info)
	}
}

func TestInitKeepsAndRestores(t *testing.T) {
	dir := initRepo(t, "main")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".lazydev/config.yaml", "# own\nbaseBranch: next\n")
	writeFile(t, dir, ".lazydev/implement.md", "own prompt\n")
	if err := os.Remove(PromptPath(dir, "review")); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, ConfigPath(dir)); got != "# own\nbaseBranch: next\n" {
		t.Errorf("config.yaml overwritten: %q", got)
	}
	if got := readFile(t, PromptPath(dir, "implement")); got != "own prompt\n" {
		t.Errorf("implement.md overwritten: %q", got)
	}
	if got := readFile(t, PromptPath(dir, "review")); got != workflowPrompt(t, "review") {
		t.Errorf("review.md not restored: %q", got)
	}
}

func TestWritePrompt(t *testing.T) {
	setupEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/merge.md", "own\n")
	if err := WritePrompt(dir, "merge"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, PromptPath(dir, "merge")); got != workflowPrompt(t, "merge") {
		t.Errorf("merge.md = %q", got)
	}
	if err := WritePrompt(dir, "deploy"); err == nil {
		t.Error("no error for an unknown step")
	}
}

func TestReadWithoutLazydev(t *testing.T) {
	setupEnv(t)
	info := Read(t.TempDir(), "p")
	if info.Initialized || info.Err != nil || info.ID != "p" || !slices.Equal(info.Prompts, []bool{false, false, false, false}) {
		t.Errorf("Read = %+v", info)
	}
}

func TestReadPromptFlags(t *testing.T) {
	setupEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/implement.md", "x")
	writeFile(t, dir, ".lazydev/merge.md", "x")
	info := Read(dir, "p")
	if info.Initialized || !slices.Equal(info.Prompts, []bool{false, true, false, true}) {
		t.Errorf("Read = %+v", info)
	}
}

func TestReadConfig(t *testing.T) {
	setupEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/config.yaml", `# settings
baseBranch: next
preset: nuxt-drizzle
knowledge:
  all: [CLAUDE.md]
  implement: [docs/db.md]
`)
	info := Read(dir, "p")
	if info.Err != nil || !info.Initialized {
		t.Fatalf("Read = %+v", info)
	}
	c := info.Config
	if c.Base() != "next" || !slices.Equal(c.Knowledge["all"], []string{"CLAUDE.md"}) ||
		!slices.Equal(c.Knowledge["implement"], []string{"docs/db.md"}) || info.Trust != TrustNone {
		t.Errorf("Config = %+v, Trust %v", c, info.Trust)
	}
}

func TestReadBrokenYAML(t *testing.T) {
	setupEnv(t)
	for _, content := range []string{"baseBranch: [unclosed\n", "allowedTools: Bash\n", "- a\n- b\n"} {
		dir := t.TempDir()
		writeFile(t, dir, ".lazydev/config.yaml", content)
		info := Read(dir, "p")
		if !info.Initialized || info.Err == nil || !strings.Contains(info.Err.Error(), "config.yaml") {
			t.Errorf("Read(%q) = %+v", content, info)
		}
	}
}

func TestReadConfigIsFolder(t *testing.T) {
	setupEnv(t)
	dir := t.TempDir()
	if err := os.MkdirAll(ConfigPath(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if info := Read(dir, "p"); info.Err == nil || info.Initialized {
		t.Errorf("Read = %+v", info)
	}
}

func TestReadTrust(t *testing.T) {
	setupEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, ".lazydev/config.yaml", "allowedTools:\n  - Bash(pnpm test:*)\nhooks:\n  setup: ./setup.sh\n")
	info := Read(dir, "p")
	if info.Trust != TrustPending {
		t.Fatalf("Trust = %v, want pending", info.Trust)
	}
	if err := state.Trust("p", info.Config.TrustHash()); err != nil {
		t.Fatal(err)
	}
	if info := Read(dir, "p"); info.Trust != TrustConfirmed {
		t.Errorf("Trust = %v, want confirmed", info.Trust)
	}
	// Another project ID has its own trust.
	if info := Read(dir, "q"); info.Trust != TrustPending {
		t.Errorf("Trust of q = %v, want pending", info.Trust)
	}
	// A changed hook needs a new confirmation.
	writeFile(t, dir, ".lazydev/config.yaml", "allowedTools:\n  - Bash(pnpm test:*)\nhooks:\n  setup: ./other.sh\n")
	if info := Read(dir, "p"); info.Trust != TrustPending {
		t.Errorf("Trust after change = %v, want pending", info.Trust)
	}
}

func TestTrustHash(t *testing.T) {
	base := Config{AllowedTools: []string{"Bash(x)"}, Hooks: Hooks{Setup: "./s.sh"}}
	h := base.TrustHash()
	if len(h) != 64 {
		t.Fatalf("TrustHash = %q", h)
	}
	other := base
	other.BaseBranch = "next"
	other.Knowledge = map[string][]string{"all": {"CLAUDE.md"}}
	if other.TrustHash() != h {
		t.Error("baseBranch or knowledge change the hash")
	}
	for name, c := range map[string]Config{
		"tools":    {AllowedTools: []string{"Bash(y)"}, Hooks: Hooks{Setup: "./s.sh"}},
		"no tools": {Hooks: Hooks{Setup: "./s.sh"}},
		"setup":    {AllowedTools: []string{"Bash(x)"}, Hooks: Hooks{Setup: "./t.sh"}},
		"teardown": {AllowedTools: []string{"Bash(x)"}, Hooks: Hooks{Setup: "./s.sh", Teardown: "./t.sh"}},
		"check":    {AllowedTools: []string{"Bash(x)"}, Hooks: Hooks{Setup: "./s.sh", Check: "./c.sh"}},
		"dev":      {AllowedTools: []string{"Bash(x)"}, Hooks: Hooks{Setup: "./s.sh", Dev: "./d.sh"}},
	} {
		if c.TrustHash() == h {
			t.Errorf("%s: same hash", name)
		}
	}
	if (Config{BaseBranch: "x"}).TrustHash() != "" || (Config{}).TrustSections() != "" {
		t.Error("hash or sections without hooks and allowed tools")
	}
	if s := (Config{AllowedTools: []string{"Bash(x)"}}).TrustSections(); s != "allowedTools:\n    - Bash(x)\n" {
		t.Errorf("TrustSections = %q", s)
	}
}

func TestTrustHashIgnoresComments(t *testing.T) {
	setupEnv(t)
	a, b := t.TempDir(), t.TempDir()
	writeFile(t, a, ".lazydev/config.yaml", "allowedTools: [Bash(x)]\nhooks: {dev: ./d.sh}\n")
	writeFile(t, b, ".lazydev/config.yaml", "# comment\nbaseBranch: next\nhooks:\n  # dev server\n  dev: ./d.sh\nallowedTools:\n  - Bash(x) # tool\nunknown: 1\n")
	ia, ib := Read(a, "p"), Read(b, "p")
	if ia.Err != nil || ib.Err != nil {
		t.Fatal(ia.Err, ib.Err)
	}
	if ia.Config.TrustHash() == "" || ia.Config.TrustHash() != ib.Config.TrustHash() {
		t.Errorf("hashes differ: %q, %q", ia.Config.TrustHash(), ib.Config.TrustHash())
	}
}

func TestBase(t *testing.T) {
	if (Config{}).Base() != "main" || (Config{BaseBranch: "next"}).Base() != "next" {
		t.Error("Base")
	}
}
