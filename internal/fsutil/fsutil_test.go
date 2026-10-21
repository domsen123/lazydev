package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a", "b", "file.json")
	if err := WriteAtomic(p, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm: %v", info.Mode().Perm())
	}
	if err := WriteAtomic(p, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) != "two" {
		t.Errorf("content: %q", data)
	}
	if info, _ := os.Stat(p); info.Mode().Perm() != 0o644 {
		t.Errorf("perm after overwrite: %v", info.Mode().Perm())
	}
	// No temporary files stay behind.
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("entries: %v", entries)
	}
}

func TestWriteAtomicError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	os.WriteFile(file, nil, 0o644)
	if err := WriteAtomic(filepath.Join(file, "x"), nil, 0o644); err == nil {
		t.Error("no error below a file")
	}
}

func TestMove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.md")
	dst := filepath.Join(dir, "new", "folder", "dst.md")
	os.WriteFile(src, []byte("prd"), 0o644)
	if err := Move(src, dst); err != nil {
		t.Fatal(err)
	}
	if Exists(src) {
		t.Error("source still there")
	}
	if data, _ := os.ReadFile(dst); string(data) != "prd" {
		t.Errorf("target: %q", data)
	}
	if err := Move(filepath.Join(dir, "missing"), filepath.Join(dir, "x")); err == nil {
		t.Error("no error for missing source")
	}
}

func TestExistsIsDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	os.WriteFile(file, nil, 0o644)
	link := filepath.Join(dir, "link")
	os.Symlink(dir, link)
	broken := filepath.Join(dir, "broken")
	os.Symlink(filepath.Join(dir, "missing"), broken)

	for _, tc := range []struct {
		path        string
		exists, dir bool
	}{
		{dir, true, true},
		{file, true, false},
		{link, true, true},
		{broken, false, false},
		{filepath.Join(dir, "missing"), false, false},
	} {
		if Exists(tc.path) != tc.exists || IsDir(tc.path) != tc.dir {
			t.Errorf("%s: Exists %v IsDir %v", tc.path, Exists(tc.path), IsDir(tc.path))
		}
	}
}
