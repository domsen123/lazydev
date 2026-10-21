package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazydev/internal/config"
	"lazydev/internal/runlog"
)

func TestParseEnv(t *testing.T) {
	vals := parseEnv("# Kommentar\nA=1\nexport B=\"zwei\"\nC='drei'\n\nkaputt\nD = 4 \n")
	want := map[string]string{"A": "1", "B": "zwei", "C": "drei", "D": "4"}
	for k, v := range want {
		if vals[k] != v {
			t.Errorf("%s = %q, erwartet %q", k, vals[k], v)
		}
	}
	if len(vals) != len(want) {
		t.Errorf("unerwartete Schlüssel: %v", vals)
	}
}

func TestSetEnv(t *testing.T) {
	in := "# DB\nNUXT_DATABASE_NAME=\"nuxt-starter\"\nOTHER=1"
	got := setEnv(in, "NUXT_DATABASE_NAME", "nuxt-starter_x")
	if got != "# DB\nNUXT_DATABASE_NAME=\"nuxt-starter_x\"\nOTHER=1" {
		t.Errorf("ersetzen: %q", got)
	}
	got = setEnv(got, "PORT", "3101")
	if !strings.HasSuffix(got, "OTHER=1\nPORT=\"3101\"\n") {
		t.Errorf("anhängen: %q", got)
	}
	if got := setEnv("", "PORT", "1"); got != "PORT=\"1\"\n" {
		t.Errorf("leere Datei: %q", got)
	}
}

func TestScriptHookOverridesPreset(t *testing.T) {
	proj := t.TempDir()
	wt := t.TempDir()
	script := filepath.Join(proj, "setup.sh")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"$LAZYDEV_SLUG:$LAZYDEV_PORT:$PWD\" > out.txt\necho hallo\n"), 0o755)

	h := For(proj, config.Repo{Preset: config.PresetNuxtDrizzle, Hooks: config.Hooks{Setup: "./setup.sh", Dev: "dev.sh"}})
	if len(h.Dev) != 1 || h.Dev[0] != filepath.Join(proj, "dev.sh") {
		t.Errorf("Dev ab dem Hauptordner: %v", h.Dev)
	}
	logPath := filepath.Join(t.TempDir(), "setup.jsonl")
	log, err := runlog.Open(logPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := Env{ProjectDir: proj, Worktree: wt, Branch: "feat/x", Port: 3101, Slug: "x", Log: log}
	if err := h.Setup(e); err != nil {
		t.Fatal(err)
	}
	log.Close()

	out, _ := os.ReadFile(filepath.Join(wt, "out.txt"))
	if strings.TrimSpace(string(out)) != "x:3101:"+wt {
		t.Errorf("Skript-Umgebung: %q", out)
	}
	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), `"text":"hallo"`) || !strings.Contains(string(data), `"type":"lazydev"`) {
		t.Errorf("Ausgabe fehlt im Log:\n%s", data)
	}
}

func TestScriptHookFailure(t *testing.T) {
	proj := t.TempDir()
	os.WriteFile(filepath.Join(proj, "fail.sh"), []byte("#!/bin/sh\nexit 3\n"), 0o755)
	log, _ := runlog.Open("", nil)
	h := For(proj, config.Repo{Hooks: config.Hooks{Teardown: "fail.sh"}})
	err := h.Teardown(Env{ProjectDir: proj, Worktree: proj, Log: log})
	if err == nil || !strings.Contains(err.Error(), "Exit-Code 3") {
		t.Errorf("Fehler = %v", err)
	}
	if err := h.Setup(Env{Log: log}); err != nil {
		t.Errorf("Setup ohne Preset und Skript: %v", err)
	}
}
