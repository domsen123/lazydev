package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"lazydev/internal/activity"
)

// activityReport prints the summary of the activity log, or its events as JSON lines with --json.
// --since takes days ("14d"), hours ("36h") or a date ("2026-10-01"); the default is 7d.
func activityReport(args []string) error {
	since := time.Now().Add(-7 * 24 * time.Hour)
	asJSON := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--since":
			if i+1 >= len(args) {
				usageError()
			}
			i++
			t, err := parseSince(args[i], time.Now())
			if err != nil {
				return err
			}
			since = t
		default:
			usageError()
		}
	}
	events, err := activity.Read(since)
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		for _, e := range events {
			if err := enc.Encode(e); err != nil {
				return err
			}
		}
		return nil
	}
	fmt.Print(activity.Report(events, since))
	return nil
}

// parseSince reads "7d", "36h" or "2006-01-02".
func parseSince(s string, now time.Time) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && strings.HasSuffix(s, "d") && n > 0 {
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "h")); err == nil && strings.HasSuffix(s, "h") && n > 0 {
		return now.Add(-time.Duration(n) * time.Hour), nil
	}
	return time.Time{}, fmt.Errorf("--since: want a number of days (7d), hours (36h) or a date (2026-10-01), got %q", s)
}
