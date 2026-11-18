package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazydev/internal/git"
	"lazydev/internal/lang"
)

// Choice is a status a user can set.
type Choice struct {
	Kind StatusKind
	// Label is its name in the UI and Word its name in commit messages. The value in the file
	// depends on the language of the project, see ValueAt.
	Label, Word string
}

// Choices are the statuses of the status picker in this order.
var Choices = []Choice{
	{StatusPlanned, "planned", "planned"},
	{StatusImplemented, "merged", "done"},
	{StatusDiscarded, "discarded", "discarded"},
}

// ValueAt is the status value of c in language l on day now. Implemented and discarded get the
// date.
func (c Choice) ValueAt(l lang.Lang, now time.Time) string {
	return StatusValue(c.Kind, l, now)
}

// ChangeStatus sets the status of the PRD rel (relative to dir) to choice c, written in language
// l. A tracked PRD gets committed alone and without trailer: the main folder must be on base and
// the file without uncommitted changes, otherwise nothing changes. An untracked PRD only changes.
func ChangeStatus(dir, base, rel string, l lang.Lang, c Choice, now time.Time) (committed bool, err error) {
	tracked := git.IsTracked(dir, rel)
	if tracked {
		if branch := git.CurrentBranch(dir); branch != base {
			return false, fmt.Errorf("main folder is on %s instead of %s", branch, base)
		}
		if len(git.Output(dir, "status", "--porcelain", "--", rel)) > 0 {
			return false, fmt.Errorf("%s has uncommitted changes", rel)
		}
	}
	path := filepath.Join(dir, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content, ok := SetStatus(string(data), c.ValueAt(l, now))
	if !ok {
		return false, fmt.Errorf("%s has no status line", rel)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	if !tracked {
		return false, nil
	}
	name := strings.TrimSuffix(filepath.Base(rel), ".md")
	if _, err := git.Run(dir, "commit", "-m", fmt.Sprintf("docs(prd): mark %s as %s", name, c.Word), "--", rel); err != nil {
		return false, err
	}
	return true, nil
}
