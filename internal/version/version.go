// Package version holds the version of lazydev and looks up the newest release on GitHub.
package version

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Version is the version of this binary. make build, make install and GoReleaser set it with
// -ldflags "-X lazydev/internal/version.Version=…", go run keeps "dev".
var Version = "dev"

// Repo is the GitHub repository of the releases.
const Repo = "domsen123/lazydev"

// GH is the gh binary. Tests set a fake script.
var GH = "gh"

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

// Latest asks gh for the tag of the newest release of Repo.
func Latest(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, GH, "release", "view", "--repo", Repo, "--json", "tagName", "--jq", ".tagName")
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
