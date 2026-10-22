package version

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// fakeGitHub serves the API and the assets of release tag with a lazydev that contains bin. sums
// replaces checksums.txt unless empty.
func fakeGitHub(t *testing.T, tag string, bin []byte, sums string) {
	t.Helper()
	name := "lazydev_" + strings.TrimPrefix(tag, "v") + "_linux_" + runtime.GOARCH + ".tar.gz"
	archive := tarGz(t, map[string][]byte{"LICENSE": []byte("MIT"), "lazydev": bin})
	if sums == "" {
		sum := sha256.Sum256(archive)
		sums = hex.EncodeToString(sum[:]) + "  " + name + "\n"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/domsen123/lazydev/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q, "name": "x"}`, tag)
	})
	mux.HandleFunc("/domsen123/lazydev/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, sums)
	})
	mux.HandleFunc("/domsen123/lazydev/releases/download/"+tag+"/"+name, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	oldAPI, oldDownload := API, Download
	API, Download = srv.URL, srv.URL
	t.Cleanup(func() { API, Download = oldAPI, oldDownload })
}

func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"LICENSE", "lazydev"} {
		data := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write(data)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestLatest(t *testing.T) {
	fakeGitHub(t, "v0.2.0", nil, "")
	got, err := Latest(context.Background())
	if err != nil || got != "v0.2.0" {
		t.Errorf("Latest = %q, %v", got, err)
	}
}

func TestLatestError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	old := API
	API = srv.URL
	t.Cleanup(func() { API = old })
	if got, err := Latest(context.Background()); err == nil {
		t.Errorf("Latest = %q without error", got)
	}
}

func TestInstall(t *testing.T) {
	fakeGitHub(t, "v0.2.0", []byte("new binary"), "")
	path := filepath.Join(t.TempDir(), "lazydev")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(context.Background(), "v0.2.0", path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new binary" {
		t.Errorf("binary %q, %v", data, err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o755 {
		t.Errorf("mode %v", info.Mode())
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	name := "lazydev_0.2.0_linux_" + runtime.GOARCH + ".tar.gz"
	fakeGitHub(t, "v0.2.0", []byte("new binary"), strings.Repeat("0", 64)+"  "+name+"\n")
	path := filepath.Join(t.TempDir(), "lazydev")
	os.WriteFile(path, []byte("old binary"), 0o755)
	err := Install(context.Background(), "v0.2.0", path)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("err: %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "old binary" {
		t.Errorf("binary %q", data)
	}
}

func TestInstallNoTag(t *testing.T) {
	if err := Install(context.Background(), "main", filepath.Join(t.TempDir(), "lazydev")); err == nil {
		t.Error("Install accepted main")
	}
}
