package bull

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestBrowseTableLine(t *testing.T) {
	now := time.Date(2026, 3, 22, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		modTime time.Time
		want    string
	}{
		{
			name:    "seconds ago",
			modTime: now.Add(-35 * time.Second),
			want:    "| [[page]] | 2026-03-22 14:59:25 • Sun • 35s ago |\n",
		},
		{
			name:    "minutes and seconds ago",
			modTime: now.Add(-5*time.Minute - 10*time.Second),
			want:    "| [[page]] | 2026-03-22 14:54:50 • Sun • 5m 10s ago |\n",
		},
		{
			name:    "hours and minutes ago",
			modTime: now.Add(-3*time.Hour - 15*time.Minute),
			want:    "| [[page]] | 2026-03-22 11:45:00 • Sun • 3h 15m ago |\n",
		},
		{
			name:    "exactly 24h ago (no relative)",
			modTime: now.Add(-24 * time.Hour),
			want:    "| [[page]] | 2026-03-21 15:00:00 • Sat |\n",
		},
		{
			name:    "older than 24h (no relative)",
			modTime: now.Add(-72 * time.Hour),
			want:    "| [[page]] | 2026-03-19 15:00:00 • Thu |\n",
		},
		{
			name:    "just now (0s ago)",
			modTime: now,
			want:    "| [[page]] | 2026-03-22 15:00:00 • Sun • 0s ago |\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := browseTableLine("[[page]]", tt.modTime, now)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("browseTableLine() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
