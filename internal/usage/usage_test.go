package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const event = `{"ts":"2026-10-20T17:00:53+02:00","type":"rate_limit_event","rate_limit_info":{"status":"allowed","resetsAt":1792515600,"rateLimitType":"five_hour","overageStatus":"rejected","overageDisabledReason":"out_of_credits","isUsingOverage":false,"unifiedWindows":{"seven_day":{"utilization":0.37,"resetsAt":1792548000},"five_hour":{"utilization":0.11,"resetsAt":1792515600}}}}`

func TestParse(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	s, ok := Parse([]byte(event), now)
	if !ok {
		t.Fatal("event not parsed")
	}
	if !s.Time.Equal(now) || s.Status != "allowed" || s.OverageStatus != "rejected" || s.IsUsingOverage {
		t.Errorf("snapshot: %+v", s)
	}
	if len(s.Windows) != 2 {
		t.Fatalf("windows: %+v", s.Windows)
	}
	five, seven := s.Windows[0], s.Windows[1]
	if five.Name != "five_hour" || five.Utilization != 0.11 || !five.ResetsAt.Equal(time.Unix(1792515600, 0)) {
		t.Errorf("five_hour: %+v", five)
	}
	if seven.Name != "seven_day" || seven.Utilization != 0.37 || !seven.ResetsAt.Equal(time.Unix(1792548000, 0)) {
		t.Errorf("seven_day: %+v", seven)
	}
}

func TestParseWindowOrder(t *testing.T) {
	line := `{"type":"rate_limit_event","rate_limit_info":{"unifiedWindows":{"zeta":{},"seven_day":{},"alpha":{},"five_hour":{}}}}`
	for range 20 {
		// Map order is random, so repeat.
		s, ok := Parse([]byte(line), time.Now())
		if !ok {
			t.Fatal("event not parsed")
		}
		var names []string
		for _, w := range s.Windows {
			names = append(names, w.Name)
		}
		if len(names) != 4 || names[0] != "five_hour" || names[1] != "seven_day" || names[2] != "alpha" || names[3] != "zeta" {
			t.Fatalf("order: %v", names)
		}
	}
}

func TestParseIgnores(t *testing.T) {
	for _, line := range []string{
		"",
		"not json",
		`{"type":"assistant"}`,
		`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"}}`,
		`{"type":"result","rate_limit_info":{"unifiedWindows":{"five_hour":{}}}}`,
	} {
		if _, ok := Parse([]byte(line), time.Now()); ok {
			t.Errorf("parsed %q", line)
		}
	}
}

func TestRecordAndLoad(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, ok, err := Load(); ok || err != nil {
		t.Fatalf("Load without file: %v, %v", ok, err)
	}
	// Other lines do not create or change the file.
	Record([]byte(`{"type":"assistant"}`))
	if _, ok, _ := Load(); ok {
		t.Fatal("other line recorded")
	}
	Record([]byte(event))
	Record([]byte(`{"type":"result"}`))
	s, ok, err := Load()
	if !ok || err != nil {
		t.Fatalf("Load: %v, %v", ok, err)
	}
	if len(s.Windows) != 2 || s.Windows[0].Name != "five_hour" || s.Windows[0].Utilization != 0.11 ||
		!s.Windows[1].ResetsAt.Equal(time.Unix(1792548000, 0)) {
		t.Errorf("windows: %+v", s.Windows)
	}
	if s.Status != "allowed" || s.OverageStatus != "rejected" || time.Since(s.Time) > time.Minute {
		t.Errorf("snapshot: %+v", s)
	}
}

func TestLoadBroken(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(path()), 0o755)
	os.WriteFile(path(), []byte("{"), 0o644)
	if _, _, err := Load(); err == nil {
		t.Error("broken file gives no error")
	}
}
