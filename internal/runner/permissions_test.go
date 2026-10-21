package runner

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"lazydev/internal/claude"
	"lazydev/internal/config"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// flagValues are the lines after each line flag in the arguments of all claude calls.
func flagValues(t *testing.T, flag string) []string {
	t.Helper()
	var values []string
	lines := strings.Split(fakeArgs(t), "\n")
	for i, line := range lines {
		if line == flag && i+1 < len(lines) {
			values = append(values, lines[i+1])
		}
	}
	if len(values) == 0 {
		t.Fatalf("claude args lack %s:\n%s", flag, fakeArgs(t))
	}
	return values
}

func saveSettings(t *testing.T, s config.Settings) {
	t.Helper()
	if err := config.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionsWithoutSettings(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	for _, mode := range flagValues(t, "--permission-mode") {
		if mode != "acceptEdits" {
			t.Errorf("permission mode %q", mode)
		}
	}
	want := strings.Join(append(slices.Clone(claude.DefaultAllowedTools), "Bash(make test:*)"), ",")
	for _, tools := range flagValues(t, "--allowedTools") {
		if tools != want {
			t.Errorf("allowed tools %q, want %q", tools, want)
		}
	}
	n := len(claude.DefaultAllowedTools) + 1
	for _, step := range []string{state.StepImplement, state.StepReview} {
		log := read(t, state.LogPath("app", prdName, step))
		line := "Permissions: acceptEdits, " + strconv.Itoa(n) + " allowed tools"
		if i, j := strings.Index(log, line), strings.Index(log, "$ "+claude.Command[0]); i < 0 || j < 0 || i > j {
			t.Errorf("%s log lacks %q before the claude call:\n%s", step, line, log)
		}
	}
}

func TestPermissionsAuto(t *testing.T) {
	target := newProject(t, "", false)
	saveSettings(t, config.Settings{PermissionMode: "auto", AllowedTools: []string{"Bash(head:*)", "Read"}})
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	for _, mode := range flagValues(t, "--permission-mode") {
		if mode != "auto" {
			t.Errorf("permission mode %q", mode)
		}
	}
	for _, tools := range flagValues(t, "--allowedTools") {
		list := strings.Split(tools, ",")
		for _, tool := range []string{"Bash(head:*)", "Read"} {
			if got := strings.Count(","+tools+",", ","+tool+","); got != 1 {
				t.Errorf("%s %d times in %v", tool, got, list)
			}
		}
	}
	log := read(t, state.LogPath("app", prdName, state.StepImplement))
	// Read is a default already, so only Bash(head:*) adds to the list.
	if line := "Permissions: auto, " + strconv.Itoa(len(claude.DefaultAllowedTools)+1) + " allowed tools"; !strings.Contains(log, line) {
		t.Errorf("implement log lacks %q:\n%s", line, log)
	}
}

func TestPermissionsOrder(t *testing.T) {
	target := newProject(t, hooksConfig, true)
	saveSettings(t, config.Settings{AllowedTools: []string{"Bash(tail:*)", "Bash(make test:*)"}})
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	want := strings.Join(append(slices.Clone(claude.DefaultAllowedTools), "Bash(tail:*)", "Bash(make test:*)"), ",")
	for _, tools := range flagValues(t, "--allowedTools") {
		if tools != want {
			t.Errorf("allowed tools %q, want %q", tools, want)
		}
	}
}

func TestPermissionsUnknownMode(t *testing.T) {
	target := newProject(t, "", false)
	saveSettings(t, config.Settings{PermissionMode: "bypassPermissions"})
	want := `settings.json: unknown permissionMode "bypassPermissions", allowed: acceptEdits, auto`
	err := Run(context.Background(), target, prdName)
	if err == nil || err.Error() != want {
		t.Fatalf("Run: %v", err)
	}
	r := loadRun(t)
	if r.Phase != project.PhaseError || r.Error == nil || *r.Error != want {
		t.Errorf("state: %+v", r)
	}
	if r.NextStep() != state.StepImplement {
		t.Errorf("next step %s", r.NextStep())
	}
	if strings.Contains(fakeArgs(t), "# lazydev implement:") {
		t.Error("claude ran for implement")
	}

	// After the fix, s continues at the failed step.
	saveSettings(t, config.Settings{PermissionMode: "auto"})
	if err := Run(context.Background(), target, prdName); err != nil {
		t.Fatal(err)
	}
	if r := loadRun(t); r.Phase != project.PhaseMerged {
		t.Errorf("phase %s", r.Phase)
	}
}
