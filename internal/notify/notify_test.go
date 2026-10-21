package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// server answers with the codes in order, the last one for every further request.
type server struct {
	*httptest.Server
	mu     sync.Mutex
	codes  []int
	bodies []string
	types  []string
}

func newServer(t *testing.T, codes ...int) *server {
	t.Helper()
	s := &server{codes: codes}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		i := min(len(s.bodies), len(s.codes)-1)
		s.bodies = append(s.bodies, string(body))
		s.types = append(s.types, r.Header.Get("Content-Type"))
		s.mu.Unlock()
		w.WriteHeader(s.codes[i])
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *server) requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

func noDelay(t *testing.T) {
	old := RetryDelay
	RetryDelay = 0
	t.Cleanup(func() { RetryDelay = old })
}

func TestPayloadEmbed(t *testing.T) {
	body, err := Payload(Message{Event: Merged, Project: "app", PRD: "auth/new-token", Branch: "feat/new-token", Base: "main",
		Model: "claude-opus-5-5", Cost: 0.8, Duration: 65 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	if p.Username != "lazydev" || len(p.Embeds) != 1 {
		t.Fatalf("payload: %s", body)
	}
	e := p.Embeds[0]
	if e.Title != "✓ Merged: auth/new-token" || e.Color != 0x2ecc71 {
		t.Errorf("title %q, color %#x", e.Title, e.Color)
	}
	if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
		t.Errorf("timestamp %q: %v", e.Timestamp, err)
	}
	want := []field{
		{"Project", "app", true},
		{"Branch", "feat/new-token → main", true},
		{"Cost", "0.80 USD", true},
		{"Duration", "1m 5s", true},
		{"Model", "claude-opus-5-5", true},
	}
	if len(e.Fields) != len(want) {
		t.Fatalf("fields: %+v", e.Fields)
	}
	for i, f := range want {
		if e.Fields[i] != f {
			t.Errorf("field %d: %+v, want %+v", i, e.Fields[i], f)
		}
	}
}

func TestPayloadTitles(t *testing.T) {
	for _, tc := range []struct {
		event Event
		title string
		color int
	}{
		{Merged, "✓ Merged: a/b", 0x2ecc71},
		{Failed, "✗ Failed: a/b", 0xe74c3c},
		{Aborted, "■ Aborted: a/b", 0x95a5a6},
		{MainFolder, "◐ Waiting for main folder: a/b", 0xf1c40f},
	} {
		body, err := Payload(Message{Event: tc.event, PRD: "a/b"})
		if err != nil {
			t.Fatal(err)
		}
		var p payload
		json.Unmarshal(body, &p)
		if e := p.Embeds[0]; e.Title != tc.title || e.Color != tc.color || e.Fields[4].Value != "–" {
			t.Errorf("%s: %+v", tc.event, e)
		}
	}
	if _, err := Payload(Message{Event: "other"}); err == nil {
		t.Error("no error for an unknown event")
	}
}

func TestTestPayload(t *testing.T) {
	for project, want := range map[string]string{"shop": "shop", "": "–"} {
		body, _ := TestPayload(project)
		var p payload
		json.Unmarshal(body, &p)
		e := p.Embeds[0]
		if e.Title != "Test from lazydev" || e.Color != 0x3498db || len(e.Fields) != 1 || e.Fields[0].Name != "Project" || e.Fields[0].Value != want {
			t.Errorf("%q: %+v", project, e)
		}
	}
}

func TestDeliverOK(t *testing.T) {
	noDelay(t)
	s := newServer(t, http.StatusNoContent)
	if err := Deliver(s.URL, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if n := s.requests(); n != 1 {
		t.Errorf("requests: %d", n)
	}
	if s.bodies[0] != `{"a":1}` || s.types[0] != "application/json" {
		t.Errorf("body %q, type %q", s.bodies[0], s.types[0])
	}
}

func TestDeliverFails(t *testing.T) {
	noDelay(t)
	s := newServer(t, http.StatusInternalServerError)
	err := Deliver(s.URL, []byte("{}"))
	if err == nil || !strings.Contains(err.Error(), "discord answered 500") {
		t.Errorf("err: %v", err)
	}
	if n := s.requests(); n != 4 {
		t.Errorf("requests: %d", n)
	}
}

func TestDeliverRetries(t *testing.T) {
	noDelay(t)
	s := newServer(t, 500, 500, 204)
	if err := Deliver(s.URL, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if n := s.requests(); n != 3 {
		t.Errorf("requests: %d", n)
	}
}

func TestPostClientError(t *testing.T) {
	s := newServer(t, http.StatusNotFound)
	if err := Post(t.Context(), s.URL, []byte("{}")); err == nil || err.Error() != "discord answered 404 Not Found" {
		t.Errorf("err: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                             "0s",
		5 * time.Second:               "5s",
		4*time.Minute + 5*time.Second: "4m 5s",
		time.Hour + 2*time.Minute + 3*time.Second: "1h 2m 3s",
		time.Hour + 3*time.Second:                 "1h 0m 3s",
		-time.Second:                              "0s",
	} {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
