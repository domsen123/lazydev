package runner

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"lazydev/internal/devserver"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// devConfig adds a dev server to the hooks. It records its port and evidence folder and runs until
// it gets stopped.
const devConfig = hooksConfig + "  dev: dev.sh\n"

const devScript = `#!/bin/sh
echo "dev $PORT $LAZYDEV_EVIDENCE" >> "$LAZYDEV_PROJECT/hooks.log"
exec sleep 30
`

func TestVerifyWritesEvidence(t *testing.T) {
	target := newProject(t, devConfig, false)
	write(t, target.Dir, "dev.sh", devScript)
	if err := os.Chmod(filepath.Join(target.Dir, "dev.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, target.Dir, "add", "dev.sh")
	runGit(t, target.Dir, "commit", "-q", "-m", "dev")
	withVerify(t, target)
	trustProject(t, target)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged {
		t.Fatalf("state: %+v", r)
	}
	if got := stepNames(r); got != "plan setup implement verify review merge" {
		t.Errorf("steps: %s", got)
	}
	if s := r.Steps[3]; !s.OK || s.CostUSD == nil || *s.CostUSD != 0.2 {
		t.Errorf("verify step: %+v", s)
	}
	ev := state.EvidenceDir("app", prdName)
	if got := read(t, filepath.Join(ev, "list.png")); got != "png\n" {
		t.Errorf("list.png: %q", got)
	}
	if got := read(t, filepath.Join(ev, "report.md")); !strings.HasPrefix(got, "## Result\nverified\n") || !strings.Contains(got, "- list.png: die Liste mit Token") {
		t.Errorf("report.md:\n%s", got)
	}
	// Claude gets the PRD, the port, the evidence folder as --add-dir and the project's prompt.
	args := fakeArgs(t)
	for _, s := range []string{"-p\n# lazydev verify: " + prdPath + "\n", "--add-dir\n" + ev + "\n",
		"Prüfe " + prdPath + " auf Port " + strconv.Itoa(r.Port) + ", Belege nach " + ev + "."} {
		if !strings.Contains(args, s) {
			t.Errorf("claude args lack %q:\n%s", s, args)
		}
	}
	// The dev server ran for verify only, with port and evidence folder.
	if got := read(t, filepath.Join(target.Dir, "hooks.log")); !strings.Contains(got, "dev "+strconv.Itoa(r.Port)+" "+ev+"\n") {
		t.Errorf("hooks.log:\n%s", got)
	}
	if devserver.PID("app", prdName) != 0 {
		t.Error("dev server still runs")
	}
	log := read(t, state.LogPath("app", prdName, state.StepVerify))
	for _, s := range []string{"Dev server started for verify", "Dev server stopped", "verify done"} {
		if !strings.Contains(log, s) {
			t.Errorf("verify log lacks %q:\n%s", s, log)
		}
	}
	// The fix of verify reached the base branch through the review.
	if got := read(t, filepath.Join(target.Dir, "verify-fix.txt")); got != "fix\n" {
		t.Errorf("verify-fix.txt: %q", got)
	}
	items, err := Overview(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Evidence != ev {
		t.Errorf("overview: %+v", items)
	}
}

func TestVerifyMissingEvidence(t *testing.T) {
	target := newProject(t, "", false)
	withVerify(t, target)
	t.Setenv("FAKE_VERIFY", "missing")
	ev := state.EvidenceDir("app", prdName)
	err := Run(context.Background(), target, prdName)
	want := "verify: evidence file list.png missing in " + ev
	if err == nil || err.Error() != want {
		t.Fatalf("Run: %v, want %s", err, want)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseError || r.Error == nil || *r.Error != want || r.NextStep() != state.StepVerify {
		t.Errorf("state: %+v", r)
	}
	// The report stays for a look.
	if got := read(t, filepath.Join(ev, "report.md")); !strings.Contains(got, "verified") {
		t.Errorf("report.md:\n%s", got)
	}

	// s repeats verify, existing evidence stays.
	t.Setenv("FAKE_VERIFY", "ok")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged {
		t.Errorf("phase: %s", r.Phase)
	}
}

func TestVerifyBlockedAndResolved(t *testing.T) {
	target := newProject(t, "", false)
	withVerify(t, target)
	autoUnblock(t, target)
	t.Setenv("FAKE_VERIFY", "blockonce")
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseMerged {
		t.Fatalf("state: %+v", r)
	}
	if got := stepNames(r); got != "plan setup implement verify resolve verify review merge" {
		t.Errorf("steps: %s", got)
	}
	logs := runGit(t, target.Dir, "log", "--format=%s")
	for _, s := range []string{"docs(prd): record blocker of new-token", "docs(prd): resolve blocker of new-token"} {
		if !strings.Contains(logs, s) {
			t.Errorf("git log lacks %q:\n%s", s, logs)
		}
	}
	// resolve set the status back to implemented, because the implementation stands.
	if prd := read(t, filepath.Join(target.Dir, prdPath)); !strings.Contains(prd, "> Status: implemented (2026-10-20)") || strings.Contains(prd, "## Blocker") {
		t.Errorf("PRD:\n%s", prd)
	}
	if log := read(t, state.LogPath("app", prdName, state.StepVerify)); !strings.Contains(log, "Blocked: verify blocked: Welcher Browser?, Claude resolves it (auto-unblock)") {
		t.Errorf("verify log:\n%s", log)
	}
}

func TestVerifyBlockedWritesBlocker(t *testing.T) {
	target := newProject(t, "", false)
	withVerify(t, target)
	t.Setenv("FAKE_VERIFY", "blockonce")
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != "verify blocked: Welcher Browser?" {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseBlocked || r.NextStep() != state.StepVerify {
		t.Errorf("state: %+v", r)
	}
	prd := read(t, filepath.Join(WorktreePath("app", prdName), prdPath))
	if !strings.HasSuffix(prd, "## Blocker\n\nStep: verify\n\n- Welcher Browser?\n") {
		t.Errorf("PRD in the worktree:\n%s", prd)
	}
}

func TestCheckVerify(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.png", "x")
	for report, want := range map[string]string{
		"## Ergebnis\nverifiziert\n\n## Belege\n- a.png: Liste\n":             "",
		"## Ergebnis\nverifiziert\n\n## Belege\n- `a.png`: Liste\n":           "",
		"## Ergebnis\nverifiziert\n\n## Belege\n- " + dir + "/a.png: Liste\n": "",
		"## Ergebnis\nverifiziert\n\n## Belege\n- keine\n":                    "verify: no evidence under „## Evidence“",
		"## Ergebnis\nverifiziert\n":                                          "verify: no evidence under „## Evidence“",
		"## Ergebnis\nverifiziert\n\n## Belege\n- a.png: x\n- b.png: y\n":     "verify: evidence file b.png missing in " + dir,
		"## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- keine\n":             "verify aborted",
		"## Ergebnis\nabgebrochen\n\n## Offene Fragen\n- Welche Seite?\n":     "verify blocked: Welche Seite?",
		"## Result\nverified\n\n## Evidence\n- a.png: list\n":                 "",
		"## Result\nverified\n\n## Evidence\n- none\n":                        "verify: no evidence under „## Evidence“",
		"## Result\naborted\n\n## Open questions\n- Which page?\n":            "verify blocked: Which page?",
		"kein Bericht":             "verify: report of verify without „## Result“",
		"## Ergebnis\numgesetzt\n": `verify: unknown result "umgesetzt" in the report of verify`,
	} {
		err := checkVerify(report, dir)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != want {
			t.Errorf("checkVerify(%q) = %q, want %q", report, got, want)
		}
	}
}
