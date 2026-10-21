package project

// Phase is the stage of a PRD. Without a run, it comes from the status line: planned, merged,
// discarded or unknown. A run sets the others.
type Phase string

const (
	PhasePlanned    Phase = "planned"
	PhaseWaiting    Phase = "waiting"
	PhaseSetup      Phase = "setup"
	PhaseImplement  Phase = "implement"
	PhaseReview     Phase = "review"
	PhaseMergeQueue Phase = "merge-queue"
	PhaseMerge      Phase = "merge"
	PhaseMerged     Phase = "merged"
	PhaseAborted    Phase = "aborted"
	PhaseError      Phase = "error"
	PhaseDiscarded  Phase = "discarded"
	PhaseUnknown    Phase = "unknown"
)

func (p Phase) Symbol() string {
	switch p {
	case PhasePlanned:
		return "○"
	case PhaseWaiting:
		return "◌"
	case PhaseSetup, PhaseImplement, PhaseReview, PhaseMerge:
		return "●"
	case PhaseMergeQueue:
		return "◐"
	case PhaseMerged:
		return "✓"
	case PhaseAborted:
		return "■"
	case PhaseError, PhaseDiscarded:
		return "✗"
	}
	return "?"
}

// Done is true for PRDs that need no more work.
func (p Phase) Done() bool {
	return p == PhaseMerged || p == PhaseDiscarded
}

// Active is true for the phases in which a runner works.
func (p Phase) Active() bool {
	switch p {
	case PhaseSetup, PhaseImplement, PhaseReview, PhaseMerge:
		return true
	}
	return false
}
