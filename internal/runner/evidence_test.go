package runner

import (
	"slices"
	"testing"
)

func TestEvidenceFilesTakesOnlyTopLevelItems(t *testing.T) {
	section := "- `/e/1-check.txt`: green\n" +
		"- 2-diff.txt: the diff shows:\n" +
		"  - All seven files have `## Routes`: as planned.\n" +
		"    continued text: still the item above\n" +
		"- keine\n" +
		"text without item: ignored\n"
	got := evidenceFiles(section, "/e")
	if want := []string{"1-check.txt", "2-diff.txt"}; !slices.Equal(got, want) {
		t.Errorf("evidenceFiles = %q, want %q", got, want)
	}
}
