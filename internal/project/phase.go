package project

// Phase is the stage of a PRD. Without a run, it comes from the status line: planned, merged,
// discarded or unknown. A run sets the others.
type Phase string

const (
	PhasePlanned    Phase = "planned"
	PhaseWaiting    Phase = "waiting"
	PhasePlan       Phase = "plan"
	PhaseResolve    Phase = "resolve"
	PhaseSetup      Phase = "setup"
	PhaseImplement  Phase = "implement"
	PhaseVerify     Phase = "verify"
	PhaseReview     Phase = "review"
	PhaseMergeQueue Phase = "merge-queue"
	PhaseMerge      Phase = "merge"
	// PhasePR waits for a human to merge the pull request of the run (merge: pr). Its runner lives
	// but takes no slot.
	PhasePR        Phase = "pr"
	PhaseMerged    Phase = "merged"
	PhaseAborted   Phase = "aborted"
	PhaseError     Phase = "error"
	PhaseBlocked   Phase = "blocked"
	PhaseDiscarded Phase = "discarded"
	PhaseUnknown   Phase = "unknown"
)

func (p Phase) Symbol() string {
	switch p {
	case PhasePlanned:
		return "○"
	case PhaseWaiting:
		return "◌"
	case PhasePlan, PhaseSetup, PhaseImplement, PhaseVerify, PhaseResolve, PhaseReview, PhaseMerge:
		return "●"
	case PhaseMergeQueue:
		return "◐"
	case PhasePR:
		return "◑"
	case PhaseMerged:
		return "✓"
	case PhaseAborted:
		return "■"
	case PhaseError, PhaseDiscarded:
		return "✗"
	case PhaseBlocked:
		return "⊘"
	}
	return "?"
}

// Done is true for PRDs that need no more work.
func (p Phase) Done() bool {
	return p == PhaseMerged || p == PhaseDiscarded
}

// Active is true for the phases in which a runner lives, so without one they show as aborted. In
// PhasePR the runner only waits for the pull request.
func (p Phase) Active() bool {
	switch p {
	case PhasePlan, PhaseSetup, PhaseImplement, PhaseVerify, PhaseResolve, PhaseReview, PhaseMerge, PhasePR:
		return true
	}
	return false
}
