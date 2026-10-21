package project

// Phase ist der Schlüssel, der im Lauf-Zustand steht. Label liefert den Anzeigetext.
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
	// PhaseDiscarded gilt für PRDs mit Statuswert "verworfen".
	PhaseDiscarded Phase = "discarded"
	// PhaseUnknown gilt für PRDs ohne lesbare Statuszeile.
	PhaseUnknown Phase = "unknown"
)

var phaseLabels = map[Phase]string{
	PhasePlanned:    "geplant",
	PhaseWaiting:    "wartet",
	PhaseSetup:      "Setup",
	PhaseImplement:  "Umsetzung",
	PhaseReview:     "Review",
	PhaseMergeQueue: "Merge-Queue",
	PhaseMerge:      "Merge",
	PhaseMerged:     "gemergt",
	PhaseAborted:    "abgebrochen",
	PhaseError:      "Fehler",
	PhaseDiscarded:  "verworfen",
	PhaseUnknown:    "unbekannt",
}

// Label liefert für unbekannte Schlüssel den Schlüssel selbst.
func (p Phase) Label() string {
	if l, ok := phaseLabels[p]; ok {
		return l
	}
	return string(p)
}

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
	default:
		return "?"
	}
}

// PhaseFromStatus gilt, solange es keinen Lauf-Zustand gibt.
func PhaseFromStatus(s Status) Phase {
	switch s.Kind {
	case StatusPlanned:
		return PhasePlanned
	case StatusImplemented:
		return PhaseMerged
	case StatusDiscarded:
		return PhaseDiscarded
	default:
		return PhaseUnknown
	}
}
