// Package version holds the version of lazydev, looks up the newest release on GitHub and installs it.
package version

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"lazydev/internal/fsutil"
)

// Version is the version of this binary. make build, make install and GoReleaser set it with
// -ldflags "-X lazydev/internal/version.Version=…", go run keeps "dev".
var Version = "dev"

// Repo is the GitHub repository of the releases.
const Repo = "domsen123/lazydev"

// API is the GitHub API and Download the host of the release assets. Tests point both at a test server.
var (
	API      = "https://api.github.com"
	Download = "https://github.com"
)

// maxDownload bounds every response, the archives have a few MB.
const maxDownload = 100 << 20

// Release parses v as vMAJOR.MINOR.PATCH. ok is false for anything else, suffixes included.
func Release(v string) (major, minor, patch int, ok bool) {
	rest, found := strings.CutPrefix(v, "v")
	if !found {
		return 0, 0, 0, false
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var nums [3]int
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return 0, 0, 0, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}

// Newer reports whether latest is a higher release than current. Versions without the release format
// are never newer.
func Newer(latest, current string) bool {
	lMajor, lMinor, lPatch, ok := Release(latest)
	if !ok {
		return false
	}
	cMajor, cMinor, cPatch, ok := Release(current)
	if !ok {
		return false
	}
	if lMajor != cMajor {
		return lMajor > cMajor
	}
	if lMinor != cMinor {
		return lMinor > cMinor
	}
	return lPatch > cPatch
}

// Latest asks the GitHub API for the tag of the newest release of Repo.
func Latest(ctx context.Context) (string, error) {
	body, err := get(ctx, API+"/repos/"+Repo+"/releases/latest")
	if err != nil {
		return "", err
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", errors.New("release without tag")
	}
	return release.TagName, nil
}

// Install downloads release tag for this architecture, checks it against checksums.txt and replaces
// the binary at path. A running lazydev keeps its old code until it restarts.
func Install(ctx context.Context, tag, path string) error {
	if _, _, _, ok := Release(tag); !ok {
		return fmt.Errorf("%q is no release tag", tag)
	}
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return fmt.Errorf("no release for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	name := "lazydev_" + tag[1:] + "_linux_" + runtime.GOARCH + ".tar.gz"
	base := Download + "/" + Repo + "/releases/download/" + tag + "/"
	sums, err := get(ctx, base+"checksums.txt")
	if err != nil {
		return err
	}
	want := checksum(sums, name)
	if want == "" {
		return fmt.Errorf("checksums.txt of %s lacks %s", tag, name)
	}
	archive, err := get(ctx, base+name)
	if err != nil {
		return err
	}
	if sum := sha256.Sum256(archive); hex.EncodeToString(sum[:]) != want {
		return fmt.Errorf("checksum mismatch for %s", name)
	}
	bin, err := extract(archive)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return fsutil.WriteAtomic(path, bin, 0o755)
}

// get fetches url and fails on any status but 200.
func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownload))
}

// checksum returns the SHA-256 of name from checksums.txt, empty if it is missing.
func checksum(sums []byte, name string) string {
	for line := range strings.Lines(string(sums)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			return fields[0]
		}
	}
	return ""
}

// extract returns the file lazydev from a tar.gz archive.
func extract(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("no file lazydev")
		}
		if err != nil {
			return nil, err
		}
		if h.Name == "lazydev" && h.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, maxDownload))
		}
	}
}
