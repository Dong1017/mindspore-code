package app

import (
	"testing"
	"time"

	"github.com/mindspore-lab/mindspore-cli/configs"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func newModelCommandTestApp() *Application {
	cfg := configs.DefaultConfig()
	cfg.Model.Key = "test-key"
	return &Application{
		EventCh: make(chan model.Event, 16),
		Config:  cfg,
	}
}

func drainUntilEventType(t *testing.T, app *Application, target model.EventType) model.Event {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	for {
		select {
		case ev := <-app.EventCh:
			if ev.Type == target {
				return ev
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for event type %s", target)
		}
	}
}
