package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazydev/internal/git"
)

// Choice is a status a user can set.
type Choice struct {
	Kind StatusKind
	// Value is the status value in the file, Label its name in the UI and Word its name in
	// commit messages.
	Value, Label, Word string
}

// Choices are the statuses of the status picker in this order.
var Choices = []Choice{
	{StatusPlanned, ValuePlanned, "planned", "planned"},
	{StatusImplemented, ValueImplemented, "merged", "done"},
	{StatusDiscarded, ValueDiscarded, "discarded", "discarded"},
}

// ValueAt is the status value of c on day now. umgesetzt and verworfen get the date.
func (c Choice) ValueAt(now time.Time) string {
	if c.Kind == StatusPlanned {
		return c.Value
	}
	return fmt.Sprintf("%s (%s)", c.Value, now.Format("2006-01-02"))
}

// ChangeStatus sets the status of the PRD rel (relative to dir) to choice c. A tracked PRD gets
// committed alone and without trailer: the main folder must be on base and the file without
// uncommitted changes, otherwise nothing changes. An untracked PRD only changes.
func ChangeStatus(dir, base, rel string, c Choice, now time.Time) (committed bool, err error) {
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
	content, ok := SetStatus(string(data), c.ValueAt(now))
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
