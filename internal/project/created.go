package project

import (
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"lazydev/internal/git"
)

// fileCreated liefert die Erstellzeit der Datei. Ohne Erstellzeit im Dateisystem nimmt sie die
// Änderungszeit.
func fileCreated(path string) time.Time {
	var stx unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_BTIME, &stx); err == nil &&
		stx.Mask&unix.STATX_BTIME != 0 {
		return time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec))
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// addedTimes liefert pro Pfad unter sub den Zeitpunkt des ersten Commits, der die Datei anlegt.
func addedTimes(dir, sub string) map[string]time.Time {
	const marker = "lazydev-commit "
	out, err := git.Run(dir, "log", "--diff-filter=A", "--relative", "--name-only",
		"--format="+marker+"%ct", "--", sub)
	if err != nil {
		return nil
	}
	added := map[string]time.Time{}
	var at time.Time
	// git log listet die neuesten Commits zuerst. Der letzte Treffer ist also der älteste.
	for _, l := range git.Lines(out) {
		if ts, ok := strings.CutPrefix(l, marker); ok {
			sec, _ := strconv.ParseInt(ts, 10, 64)
			at = time.Unix(sec, 0)
			continue
		}
		added[l] = at
	}
	return added
}
