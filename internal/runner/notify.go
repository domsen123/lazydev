package runner

import (
	"strings"
	"time"

	"lazydev/internal/config"
	"lazydev/internal/notify"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

// notifyRun posts event of run r to the Discord webhook of the project and logs the outcome. It
// reads settings.json every time, so a change applies to running runners too. Without a webhook it
// does nothing. A failed delivery changes neither the run nor the step.
func notifyRun(t Target, r *state.Run, event notify.Event, log *logger) {
	url := config.LoadSettings().Discord.Target(t.Dir)
	if url == "" {
		return
	}
	var duration time.Duration
	if started, err := time.Parse(time.RFC3339, r.StartedAt); err == nil {
		duration = time.Since(started)
	}
	body, err := notify.Payload(notify.Message{
		Event:    event,
		Project:  t.ID,
		PRD:      strings.TrimSuffix(strings.TrimPrefix(r.PRD, project.Dir+"/"), ".md"),
		Branch:   r.Branch,
		Base:     r.Base,
		Model:    r.ImplementModel(),
		Cost:     r.Cost(),
		Duration: duration,
	})
	if err == nil {
		err = notify.Deliver(url, body)
	}
	if err != nil {
		log.printf("Discord notification failed: %v", err)
		return
	}
	log.printf("Discord notification sent: %s", event)
}
