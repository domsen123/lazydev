package usage

import (
	"testing"
	"time"
)

const event = `{"ts":"2026-10-20T17:00:53+02:00","type":"rate_limit_event","rate_limit_info":{"status":"allowed","resetsAt":1792515600,"rateLimitType":"five_hour","overageStatus":"rejected","overageDisabledReason":"out_of_credits","isUsingOverage":false,"unifiedWindows":{"seven_day":{"utilization":0.37,"resetsAt":1792548000},"five_hour":{"utilization":0.11,"resetsAt":1792515600}}}}`

func TestRecordAndLoad(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, ok, err := Load(); ok || err != nil {
		t.Fatalf("Load ohne Datei: %v, %v", ok, err)
	}
	Record([]byte(`{"type":"assistant"}`))
	Record([]byte(event))
	s, ok, err := Load()
	if !ok || err != nil {
		t.Fatalf("Load: %v, %v", ok, err)
	}
	if len(s.Windows) != 2 || s.Windows[0].Name != "five_hour" || s.Windows[0].Utilization != 0.11 {
		t.Errorf("Fenster: %+v", s.Windows)
	}
	if w, _ := s.Window("seven_day"); !w.ResetsAt.Equal(time.Unix(1792548000, 0)) {
		t.Errorf("seven_day: %+v", w)
	}
	if s.Status != "allowed" || s.OverageStatus != "rejected" || s.OverageReason != "out_of_credits" || time.Since(s.Time) > time.Minute {
		t.Errorf("Stand: %+v", s)
	}
}
