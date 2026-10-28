package database

import (
	"errors"
	"strings"
	"testing"
)

func TestParseEnv(t *testing.T) {
	got := parseEnv("# comment\nexport A=1\nB = \"two words\"\nC='x'\nD=\"say \\\"hi\\\"\"\nnoequals\n\nE=\n")
	want := map[string]string{"A": "1", "B": "two words", "C": "x", "D": `say "hi"`, "E": ""}
	if len(got) != len(want) {
		t.Errorf("parseEnv = %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestKeys(t *testing.T) {
	got := Keys("# x\nB=1\nexport A=2\nB=3\n\nnoequals\nC=\n")
	if strings.Join(got, ",") != "B,A,C" {
		t.Errorf("Keys = %v", got)
	}
}

func TestSetEnv(t *testing.T) {
	for _, tc := range []struct{ in, key, value, want string }{
		{"A=1\nDB=app\n# DB=old\nB=2\n", "DB", "app_x", "A=1\nDB=\"app_x\"\n# DB=old\nB=2\n"},
		{"export DB='app'\n", "DB", "app_x", "DB=\"app_x\"\n"},
		{"A=1", "DB", "app_x", "A=1\nDB=\"app_x\"\n"},
		{"", "DB", "app_x", "DB=\"app_x\"\n"},
		{"DB=a\nDB=b\n", "DB", "c", "DB=\"c\"\nDB=\"c\"\n"},
		// A quote in the value must not break the file.
		{"URL=x\n", "URL", `postgres://u:p"w@h/app_x`, "URL=\"postgres://u:p\\\"w@h/app_x\"\n"},
	} {
		got := setEnv(tc.in, tc.key, tc.value)
		if got != tc.want {
			t.Errorf("setEnv(%q, %s, %q) = %q, want %q", tc.in, tc.key, tc.value, got, tc.want)
		}
		if v := parseEnv(got)[tc.key]; v != tc.value {
			t.Errorf("%q reads back as %q", got, v)
		}
	}
}

func TestBaseName(t *testing.T) {
	for value, want := range map[string]string{
		"app":   "app",
		"my_db": "my_db",
		"postgres://app:secret@localhost:5432/app?sslmode=disable": "app",
		"postgresql://localhost/shop":                              "shop",
		"postgres://u:p%40ss@db:5432/main_db#x":                    "main_db",
	} {
		if got, err := BaseName(value); err != nil || got != want {
			t.Errorf("BaseName(%q) = %q, %v, want %q", value, got, err, want)
		}
	}
	for _, value := range []string{"postgres://app:secret@localhost:5432", "postgres://localhost:5432/", "postgres://h/?sslmode=disable"} {
		if _, err := BaseName(value); !errors.Is(err, errNoName) {
			t.Errorf("BaseName(%q) error = %v", value, err)
		}
	}
}

func TestReplaceName(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"app", "app_login_form"},
		{"postgres://app:secret@localhost:5432/app?sslmode=disable", "postgres://app:secret@localhost:5432/app_login_form?sslmode=disable"},
		{"postgres://app:s%3Ac!ret@localhost:5432/app", "postgres://app:s%3Ac!ret@localhost:5432/app_login_form"},
		{"postgres://localhost:5432/app#frag", "postgres://localhost:5432/app_login_form#frag"},
		{"postgres://localhost:5432", "postgres://localhost:5432/app_login_form"},
		{"postgres://localhost:5432?sslmode=disable", "postgres://localhost:5432/app_login_form?sslmode=disable"},
	} {
		if got := ReplaceName(tc.value, "app_login_form"); got != tc.want {
			t.Errorf("ReplaceName(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestWorktreeNameTooLong(t *testing.T) {
	slug := strings.Repeat("s", 40)
	db := dbEntry("db", ".env", "DB")
	if _, err := worktreeName(db, strings.Repeat("a", 22), slug); err != nil {
		t.Errorf("63 bytes: %v", err)
	}
	name := strings.Repeat("a", 23) + "_" + slug
	if _, err := worktreeName(db, strings.Repeat("a", 23), slug); err == nil || err.Error() != "database name "+name+" is longer than 63 bytes" {
		t.Errorf("64 bytes: %v", err)
	}
	if _, err := worktreeName(db, "", slug); err == nil || err.Error() != ".env: DB is not set" {
		t.Errorf("empty: %v", err)
	}
	if _, err := worktreeName(db, "postgres://h:1", slug); err == nil || err.Error() != ".env: DB has no database name" {
		t.Errorf("no name: %v", err)
	}
}
