package gui

import (
	"testing"
	"time"

	"github.com/dahui/z13ctl/api"
)

func TestPresentTabletStatus(t *testing.T) {
	now := time.Unix(1_000, 0)
	tests := []struct {
		name      string
		status    *api.TabletIntegration
		visible   bool
		warning   bool
		posture   string
		reason    string
		heartbeat string
	}{
		{name: "missing"},
		{name: "never seen", status: &api.TabletIntegration{Posture: "tablet", Healthy: true}},
		{
			name:      "healthy at timeout boundary",
			status:    &api.TabletIntegration{Posture: "laptop", Healthy: true, LastSeen: 925},
			visible:   true,
			posture:   "Laptop",
			heartbeat: "Heartbeat 75 seconds ago",
		},
		{
			name:      "stale",
			status:    &api.TabletIntegration{Posture: "desktop", Healthy: true, LastSeen: 924},
			visible:   true,
			warning:   true,
			posture:   "Desktop",
			reason:    "Heartbeat is 76 seconds old",
			heartbeat: "Heartbeat 76 seconds ago",
		},
		{
			name:      "explicit error is exact",
			status:    &api.TabletIntegration{Posture: "tablet", Healthy: true, Error: "touch-scroll exited with status 1", LastSeen: 999},
			visible:   true,
			warning:   true,
			posture:   "Tablet",
			reason:    "touch-scroll exited with status 1",
			heartbeat: "Heartbeat 1 second ago",
		},
		{
			name:      "unhealthy without error",
			status:    &api.TabletIntegration{Posture: "tablet", LastSeen: 1_000},
			visible:   true,
			warning:   true,
			posture:   "Tablet",
			reason:    "Integration reported unhealthy",
			heartbeat: "Heartbeat just now",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := presentTabletStatus(tt.status, now)
			if got.visible != tt.visible || got.warning != tt.warning || got.posture != tt.posture || got.reason != tt.reason || got.heartbeat != tt.heartbeat {
				t.Fatalf("presentation = %+v", got)
			}
		})
	}
}
