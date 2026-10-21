package terminal

import (
	"strings"
	"testing"
)

func newFilter() (*titleFilter, *[]string) {
	var titles []string
	return &titleFilter{onTitle: func(s string) { titles = append(titles, s) }}, &titles
}

func TestTitleFilterRemovesTitles(t *testing.T) {
	f, titles := newFilter()
	in := "a\x1b]0;one\x07b\x1b]2;two\x1b\\c\x1b]1;icon\x07d"
	if got := string(f.filter([]byte(in))); got != "abcd" {
		t.Errorf("output: %q", got)
	}
	if strings.Join(*titles, "|") != "one|two|icon" {
		t.Errorf("titles: %q", *titles)
	}
}

func TestTitleFilterClaudeTitle(t *testing.T) {
	f, titles := newFilter()
	// ✳ is E2 9C B3, and 0x9C must not end the sequence.
	got := f.filter([]byte("x\x1b]0;✳ Claude Code\x07y"))
	if string(got) != "xy" || len(*titles) != 1 || (*titles)[0] != "✳ Claude Code" {
		t.Errorf("output %q, titles %q", got, *titles)
	}
}

func TestTitleFilterPassesOtherOSC(t *testing.T) {
	f, titles := newFilter()
	in := "\x1b]8;;https://example.com\x07link\x1b]8;;\x07 \x1b]7;file:///tmp\x1b\\\x1b[31mred"
	if got := string(f.filter([]byte(in))); got != in {
		t.Errorf("output: %q", got)
	}
	if len(*titles) != 0 {
		t.Errorf("titles: %q", *titles)
	}
}

func TestTitleFilterSplitSequence(t *testing.T) {
	f, titles := newFilter()
	var out strings.Builder
	for _, part := range []string{"a\x1b", "]0;sp", "lit", "\x1b", "\\b"} {
		out.Write(f.filter([]byte(part)))
	}
	if out.String() != "ab" || len(*titles) != 1 || (*titles)[0] != "split" {
		t.Errorf("output %q, titles %q", out.String(), *titles)
	}
}

func TestTitleFilterTrailingEsc(t *testing.T) {
	f, _ := newFilter()
	if got := string(f.filter([]byte("text\x1b"))); got != "text" {
		t.Errorf("output: %q", got)
	}
	if string(f.pending) != "\x1b" {
		t.Errorf("pending: %q", f.pending)
	}
	// The ESC starts a CSI, not an OSC, and comes out with the next read.
	if got := string(f.filter([]byte("[1mbold"))); got != "\x1b[1mbold" {
		t.Errorf("next read: %q", got)
	}
	if f.pending != nil {
		t.Errorf("pending after next read: %q", f.pending)
	}
}

func TestTitleFilterUnfinishedSequence(t *testing.T) {
	f, titles := newFilter()
	if got := f.filter([]byte("a\x1b]2;never ends")); string(got) != "a" {
		t.Errorf("output: %q", got)
	}
	if string(f.pending) != "\x1b]2;never ends" || len(*titles) != 0 {
		t.Errorf("pending %q, titles %q", f.pending, *titles)
	}
	// A sequence longer than maxOSC is passed through unchanged.
	long := strings.Repeat("x", maxOSC)
	got := f.filter([]byte(long))
	if string(got) != "\x1b]2;never ends"+long || f.pending != nil {
		t.Errorf("long sequence: %d bytes, pending %d", len(got), len(f.pending))
	}
}

func TestTitleFilterWithoutCallback(t *testing.T) {
	f := &titleFilter{}
	if got := string(f.filter([]byte("\x1b]0;t\x07ok"))); got != "ok" {
		t.Errorf("output: %q", got)
	}
}
