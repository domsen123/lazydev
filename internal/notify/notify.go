// Package notify posts the notifications of runners to a Discord webhook: the payload of an embed
// and its delivery with retries.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Event is what a notification reports.
type Event string

const (
	Merged     Event = "merged"
	Failed     Event = "failed"
	Aborted    Event = "aborted"
	MainFolder Event = "main folder"
)

// Timeout limits one attempt.
var Timeout = 10 * time.Second

// RetryDelay is the pause between two attempts of Deliver. Tests set it to 0.
var RetryDelay = 5 * time.Second

// attempts is the first attempt plus the retries of Deliver.
const attempts = 4

// Message is the content of a notification about a run. PRD is "<area>/<name>".
type Message struct {
	Event                             Event
	Project, PRD, Branch, Base, Model string
	Cost                              float64
	Duration                          time.Duration
}

// titles holds the title prefix and the color of each event.
var titles = map[Event]struct {
	prefix string
	color  int
}{
	Merged:     {"✓ Merged: ", 0x2ecc71},
	Failed:     {"✗ Failed: ", 0xe74c3c},
	Aborted:    {"■ Aborted: ", 0x95a5a6},
	MainFolder: {"◐ Waiting for main folder: ", 0xf1c40f},
}

// testColor is the color of the test message.
const testColor = 0x3498db

type payload struct {
	Username string  `json:"username"`
	Embeds   []embed `json:"embeds"`
}

type embed struct {
	Title     string  `json:"title"`
	Color     int     `json:"color"`
	Fields    []field `json:"fields"`
	Timestamp string  `json:"timestamp"`
}

type field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// Payload is the webhook body of m.
func Payload(m Message) ([]byte, error) {
	t, ok := titles[m.Event]
	if !ok {
		return nil, fmt.Errorf("unknown event %q", m.Event)
	}
	model := m.Model
	if model == "" {
		model = "–"
	}
	return build(t.prefix+m.PRD, t.color, []field{
		{"Project", m.Project, true},
		{"Branch", m.Branch + " → " + m.Base, true},
		{"Cost", fmt.Sprintf("%.2f USD", m.Cost), true},
		{"Duration", FormatDuration(m.Duration), true},
		{"Model", model, true},
	})
}

// TestPayload is the body of the test message for project, "–" without a project.
func TestPayload(project string) ([]byte, error) {
	if project == "" {
		project = "–"
	}
	return build("Test from lazydev", testColor, []field{{"Project", project, true}})
}

func build(title string, color int, fields []field) ([]byte, error) {
	return json.Marshal(payload{Username: "lazydev", Embeds: []embed{{
		Title: title, Color: color, Fields: fields, Timestamp: time.Now().Format(time.RFC3339),
	}}})
}

// Post sends body to url once, within Timeout. Every status outside 2xx is an error.
func Post(ctx context.Context, url string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("discord answered %s", resp.Status)
	}
	return nil
}

// Deliver posts body to url with up to 3 retries, RetryDelay apart. Every error retries, 4xx too.
// It does not take a context: an aborted run still reports its abort.
func Deliver(url string, body []byte) error {
	var err error
	for i := range attempts {
		if i > 0 {
			time.Sleep(RetryDelay)
		}
		if err = Post(context.Background(), url, body); err == nil {
			return nil
		}
	}
	return err
}

// FormatDuration writes d as "1h 2m 3s" without leading zero parts, e.g. "4m 5s" or "0s".
func FormatDuration(d time.Duration) string {
	s := max(0, int(d.Round(time.Second)/time.Second))
	h, m := s/3600, s%3600/60
	s %= 60
	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if h > 0 || m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	parts = append(parts, fmt.Sprintf("%ds", s))
	return strings.Join(parts, " ")
}
