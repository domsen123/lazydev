// Package lang holds the words lazydev reads from and writes into PRDs and reports, per language.
// A project picks its language with "language" in .lazydev/config.yaml. lazydev writes in that
// language only but always reads every language, so a project can switch without breaking old PRDs.
package lang

import (
	"fmt"
	"reflect"
)

// Lang is the language of a project's contracts, templates, agents and PRD words.
type Lang string

const (
	EN Lang = "en"
	DE Lang = "de"
	// Default applies without "language" in config.yaml.
	Default = EN
)

// Parse reads the value of "language". Empty gives Default, other values than en and de are an
// error.
func Parse(s string) (Lang, error) {
	switch Lang(s) {
	case "":
		return Default, nil
	case EN, DE:
		return Lang(s), nil
	}
	return "", fmt.Errorf("language must be en or de, not %q", s)
}

// Vocab holds the words of one language that lazydev reads or writes.
type Vocab struct {
	Lang Lang
	// Status values of the status line. Implemented, Discarded and Blocked get a date.
	Planned, Implemented, Discarded, Blocked string
	// BlockerHeading is the section of a blocked PRD, BlockerStep the label of its first line.
	BlockerHeading, BlockerStep string
	// Cost and Model are the fields of the detail the merge writes, e.g. "Cost: 4.12 USD".
	Cost, Model string
	// Report headings without "## ".
	Result, OpenQuestions, OpenPoints, Evidence, Decisions string
	// Values under the heading Result.
	ResultImplemented, ResultVerified, ResultAborted, ResultMergeReady, ResultResolved string
	// None is the point of an empty report list.
	None string
	// The answer of plan: heading without "## ", the labels of its lines.
	Order, After, Reason string
	// RequiredReading is the heading of the knowledge block without "## ", RequiredReadingText the
	// sentence below it.
	RequiredReading, RequiredReadingText string
	// CheckFailed starts the context of the Claude step merge after a red check.
	CheckFailed string
	// NoDescription is $ARGUMENTS of "lazydev prompt create" without a description, AskForTask
	// that of n in the TUI with an empty input.
	NoDescription, AskForTask string
}

var vocabs = []Vocab{
	{
		Lang:                EN,
		Planned:             "planned",
		Implemented:         "implemented",
		Discarded:           "discarded",
		Blocked:             "blocked",
		BlockerHeading:      "## Blocker",
		BlockerStep:         "Step:",
		Cost:                "Cost:",
		Model:               "Model:",
		Result:              "Result",
		OpenQuestions:       "Open questions",
		OpenPoints:          "Open points",
		Evidence:            "Evidence",
		Decisions:           "Decisions",
		ResultImplemented:   "implemented",
		ResultVerified:      "verified",
		ResultAborted:       "aborted",
		ResultMergeReady:    "ready for fast-forward",
		ResultResolved:      "resolved",
		None:                "none",
		Order:               "Order",
		After:               "After:",
		Reason:              "Reason:",
		RequiredReading:     "Required reading",
		RequiredReadingText: "Read these files completely before you do anything else:",
		CheckFailed:         "Check failed after the rebase:",
		NoDescription:       "The task follows from the conversation with the user so far.",
		AskForTask:          "New PRD. Ask me what it is about.",
	},
	{
		Lang:                DE,
		Planned:             "geplant",
		Implemented:         "umgesetzt",
		Discarded:           "verworfen",
		Blocked:             "blockiert",
		BlockerHeading:      "## Blocker",
		BlockerStep:         "Schritt:",
		Cost:                "Kosten:",
		Model:               "Modell:",
		Result:              "Ergebnis",
		OpenQuestions:       "Offene Fragen",
		OpenPoints:          "Offene Punkte",
		Evidence:            "Belege",
		Decisions:           "Entscheidungen",
		ResultImplemented:   "umgesetzt",
		ResultVerified:      "verifiziert",
		ResultAborted:       "abgebrochen",
		ResultMergeReady:    "bereit für fast-forward",
		ResultResolved:      "geklärt",
		None:                "keine",
		Order:               "Reihenfolge",
		After:               "Nach:",
		Reason:              "Grund:",
		RequiredReading:     "Pflichtlektüre",
		RequiredReadingText: "Lies diese Dateien vollständig, bevor du etwas anderes tust:",
		CheckFailed:         "Prüfung nach dem Rebase fehlgeschlagen:",
		NoDescription:       "Die Aufgabe ergibt sich aus dem bisherigen Gespräch mit dem Nutzer.",
		AskForTask:          "Neues PRD. Frag mich, worum es geht.",
	},
}

// For returns the words of l, those of Default for an unknown language.
func For(l Lang) Vocab {
	for _, v := range vocabs {
		if v.Lang == l {
			return v
		}
	}
	return vocabs[0]
}

// All returns the words of every language, Default first. Reading uses them all.
func All() []Vocab {
	return append([]Vocab(nil), vocabs...)
}

// Unique lists the words of l that no other language uses, e.g. "geplant" and "Nach:" for DE.
// Tests use it to make sure a text of one language holds no words of another.
func Unique(l Lang) []string {
	own := For(l)
	ownV := reflect.ValueOf(own)
	var words []string
	for i := range ownV.NumField() {
		w, ok := ownV.Field(i).Interface().(string)
		if !ok || ownV.Type().Field(i).Name == "Lang" {
			continue
		}
		shared := false
		for _, v := range vocabs {
			if v.Lang != l && reflect.ValueOf(v).Field(i).String() == w {
				shared = true
			}
		}
		if !shared {
			words = append(words, w)
		}
	}
	return words
}

// Langs lists the supported languages in the order of All.
func Langs() []Lang {
	var ls []Lang
	for _, v := range vocabs {
		ls = append(ls, v.Lang)
	}
	return ls
}
