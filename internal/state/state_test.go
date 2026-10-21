package state

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"lazydev/internal/project"
)

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

func TestDirHonorsXDG(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)
	if Dir() != filepath.Join(base, "lazydev") {
		t.Errorf("Dir: %s", Dir())
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/u")
	if Dir() != "/home/u/.local/state/lazydev" {
		t.Errorf("Dir without XDG_STATE_HOME: %s", Dir())
	}
}

func TestSaveLoadRemove(t *testing.T) {
	isolate(t)
	if _, err := Load("p", "x"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load without run: %v", err)
	}
	r := &Run{ProjectDir: "/p", PRD: "prd/a/x.md", Branch: "feat/x", Base: "main", Port: 3001,
		Phase: project.PhaseImplement, PID: 42, StartedAt: "2026-10-20T10:00:00Z"}
	r.StartStep(StepSetup)
	r.EndStep(0, true)
	if err := Save("p", "x", r); err != nil {
		t.Fatal(err)
	}
	got, err := Load("p", "x")
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "feat/x" || got.Phase != project.PhaseImplement || len(got.Steps) != 1 ||
		got.Steps[0].ExitCode == nil || *got.Steps[0].ExitCode != 0 || !got.Steps[0].OK {
		t.Errorf("roundtrip: %+v", got)
	}

	files := []string{RunnerLogPath("p", "x"), DevPIDPath("p", "x"), DevLogPath("p", "x")}
	for _, step := range Steps {
		files = append(files, LogPath("p", "x", step))
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other := LogPath("p", "y", StepSetup)
	os.WriteFile(other, []byte("x"), 0o644)
	if err := Remove("p", "x"); err != nil {
		t.Fatal(err)
	}
	for _, f := range append(files, Path("p", "x")) {
		if _, err := os.Stat(f); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s still there: %v", f, err)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("log of another PRD removed: %v", err)
	}
	// Removing twice is fine.
	if err := Remove("p", "x"); err != nil {
		t.Error(err)
	}
}

func TestAllAndProjectRuns(t *testing.T) {
	isolate(t)
	Save("b", "late", &Run{StartedAt: "2026-10-20T12:00:00Z"})
	Save("a", "early", &Run{StartedAt: "2026-10-20T10:00:00Z"})
	Save("a", "mid", &Run{StartedAt: "2026-10-20T11:00:00Z"})
	os.WriteFile(Path("a", "broken"), []byte("{"), 0o644)

	all := All()
	var names []string
	for _, e := range all {
		names = append(names, e.ID+"/"+e.PRDName)
	}
	if len(names) != 3 || names[0] != "a/early" || names[1] != "a/mid" || names[2] != "b/late" {
		t.Errorf("All: %v", names)
	}
	runs := ProjectRuns("a")
	if len(runs) != 2 || runs["early"] == nil || runs["mid"] == nil {
		t.Errorf("ProjectRuns: %v", runs)
	}
	if len(ProjectRuns("none")) != 0 {
		t.Error("runs for unknown project")
	}
}

func TestNextStep(t *testing.T) {
	r := &Run{}
	if got := r.NextStep(); got != StepSetup {
		t.Errorf("without steps: %q", got)
	}
	r.StartStep(StepSetup)
	if got := r.NextStep(); got != StepSetup {
		t.Errorf("unfinished step: %q", got)
	}
	r.EndStep(0, true)
	if got := r.NextStep(); got != StepImplement {
		t.Errorf("after setup: %q", got)
	}
	r.StartStep(StepImplement)
	r.EndStep(0, false)
	if got := r.NextStep(); got != StepImplement {
		t.Errorf("failed despite exit 0: %q", got)
	}
	r.StartStep(StepImplement)
	r.EndStep(1, false)
	if got := r.NextStep(); got != StepImplement {
		t.Errorf("failed step: %q", got)
	}
	r.StartStep(StepImplement)
	r.EndStep(0, true)
	r.StartStep(StepReview)
	r.EndStep(0, true)
	if got := r.NextStep(); got != StepMerge {
		t.Errorf("after review: %q", got)
	}
	r.StartStep(StepMerge)
	r.EndStep(0, true)
	if got := r.NextStep(); got != "" {
		t.Errorf("after merge: %q", got)
	}
	unknown := &Run{Steps: []Step{{Name: "other", ExitCode: new(int), OK: true}}}
	if got := unknown.NextStep(); got != "" {
		t.Errorf("unknown step: %q", got)
	}
}

func TestLastStepAndFail(t *testing.T) {
	r := &Run{Phase: project.PhaseReview}
	if r.LastStep() != nil {
		t.Error("LastStep without steps")
	}
	// EndStep without a step does nothing.
	r.EndStep(1, false)
	r.Fail(errors.New("boom"))
	if r.Phase != project.PhaseError || r.Error == nil || *r.Error != "boom" {
		t.Errorf("Fail: %+v", r)
	}
}

func TestEffective(t *testing.T) {
	// The PID of this test is no runner, its command line is not "lazydev run".
	r := &Run{Phase: project.PhaseImplement, PID: os.Getpid()}
	if r.Running() || r.Effective() != project.PhaseAborted {
		t.Errorf("active without runner: running %v, %s", r.Running(), r.Effective())
	}
	for _, p := range []project.Phase{project.PhaseWaiting, project.PhaseMergeQueue, project.PhaseMerged, project.PhaseError} {
		if got := (&Run{Phase: p}).Effective(); got != p {
			t.Errorf("%s became %s", p, got)
		}
	}
	if (&Run{Phase: project.PhaseSetup}).Running() {
		t.Error("PID 0 runs")
	}
}

// TestIsRunner starts a process whose command line is "<dir>/lazydev run": a link named lazydev
// to sh, which reads the script "run" and waits for input.
func TestIsRunner(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	link := filepath.Join(dir, BinaryName)
	if err := os.Symlink(sh, link); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "run"), []byte("read x\n"), 0o644)
	cmd := exec.Command(link, "run")
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	})
	if !IsRunner(cmd.Process.Pid) {
		data, _ := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/cmdline")
		t.Errorf("runner not recognized, cmdline %q", data)
	}
	r := &Run{Phase: project.PhaseReview, PID: cmd.Process.Pid}
	if r.Effective() != project.PhaseReview {
		t.Errorf("living runner: %s", r.Effective())
	}
	if IsRunner(os.Getpid()) || IsRunner(0) || IsRunner(-1) {
		t.Error("non-runner recognized")
	}
}

func TestCostAndImplementModel(t *testing.T) {
	c1, c2 := 0.25, 1.5
	r := &Run{Steps: []Step{
		{Name: StepSetup, OK: true},
		{Name: StepImplement, Model: "opus", OK: true, CostUSD: &c1},
		{Name: StepImplement, Model: "haiku", OK: false},
		{Name: StepReview, Model: "sonnet", OK: true, CostUSD: &c2},
	}}
	if got := r.Cost(); got != 1.75 {
		t.Errorf("Cost: %v", got)
	}
	if got := r.ImplementModel(); got != "opus" {
		t.Errorf("ImplementModel: %q", got)
	}
	if got := (&Run{}).ImplementModel(); got != "" {
		t.Errorf("ImplementModel without steps: %q", got)
	}
	if got := (&Run{}).Cost(); got != 0 {
		t.Errorf("Cost without steps: %v", got)
	}
}

func TestLock(t *testing.T) {
	isolate(t)
	unlock, err := Lock("start")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(Dir(), "locks", "start.lock")); err != nil {
		t.Error(err)
	}
	second, err := TryLock("start")
	if err != nil || second != nil {
		t.Fatalf("TryLock while held: %v, %v", second != nil, err)
	}
	other, err := TryLock("other")
	if err != nil || other == nil {
		t.Fatalf("TryLock of another lock: %v", err)
	}
	other()
	unlock()
	again, err := TryLock("start")
	if err != nil || again == nil {
		t.Fatalf("TryLock after unlock: %v", err)
	}
	again()
}

func TestTrust(t *testing.T) {
	isolate(t)
	if !Trusted("p", "") {
		t.Error("empty hash not trusted")
	}
	if Trusted("p", "abc") {
		t.Error("trusted without file")
	}
	if err := Trust("p", "abc"); err != nil {
		t.Fatal(err)
	}
	if !Trusted("p", "abc") || Trusted("p", "def") || Trusted("q", "abc") {
		t.Error("trust after Trust")
	}
	os.WriteFile(TrustPath("p"), []byte("{"), 0o644)
	if Trusted("p", "abc") {
		t.Error("broken trust file trusted")
	}
}

func TestDevPaths(t *testing.T) {
	isolate(t)
	if filepath.Dir(DevPIDPath("p", "x")) != RunsDir("p") || filepath.Base(DevLogPath("p", "x")) != "x.dev.log" {
		t.Errorf("dev paths: %s, %s", DevPIDPath("p", "x"), DevLogPath("p", "x"))
	}
}
