package bull

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestBrowseTableLine(t *testing.T) {
	tests := []struct {
		name    string
		modTime time.Time
		want    string
	}{
		{
			name:    "recent timestamp",
			modTime: time.Date(2026, 3, 22, 14, 59, 25, 0, time.UTC),
			want:    "| [[page]] | <time datetime=\"2026-03-22T14:59:25Z\">2026-03-22 14:59:25 • Sun</time> |\n",
		},
		{
			name:    "older timestamp",
			modTime: time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC),
			want:    "| [[page]] | <time datetime=\"2026-03-19T15:00:00Z\">2026-03-19 15:00:00 • Thu</time> |\n",
		},
		{
			name:    "zero value time",
			modTime: time.Time{},
			want:    "| [[page]] | <time datetime=\"0001-01-01T00:00:00Z\">0001-01-01 00:00:00 • Mon</time> |\n",
		},
		{
			name:    "non-UTC timezone normalized to RFC3339",
			modTime: time.Date(2026, 6, 15, 10, 30, 0, 0, time.FixedZone("CET", 3600)),
			want:    "| [[page]] | <time datetime=\"2026-06-15T10:30:00+01:00\">2026-06-15 10:30:00 • Mon</time> |\n",
		},
		{
			name:    "midnight boundary",
			modTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			want:    "| [[page]] | <time datetime=\"2026-01-01T00:00:00Z\">2026-01-01 00:00:00 • Thu</time> |\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := browseTableLine("[[page]]", tt.modTime)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("browseTableLine() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBrowseTableLineDeterministic(t *testing.T) {
	// browseTableLine must produce identical output for the same input,
	// regardless of when it is called. This is required for content hashing.
	modTime := time.Date(2026, 3, 22, 14, 59, 25, 0, time.UTC)
	first := browseTableLine("[[page]]", modTime)
	second := browseTableLine("[[page]]", modTime)
	if first != second {
		t.Errorf("browseTableLine is not deterministic:\n  first:  %q\n  second: %q", first, second)
	}
}
