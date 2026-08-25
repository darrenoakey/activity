package ui

import (
	"testing"

	"activity/pkg/proc"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KB"},
		{1536, "2 KB"},
		{1048576, "1 MB"},
		{1572864, "2 MB"},
		{1073741824, "1.0 GB"},
		{2147483648, "2.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestCPUColor(t *testing.T) {
	tests := []struct {
		cpu    float64
		hidden bool
		want   string
	}{
		{0.5, false, "gray"},
		{5.0, false, "green"},
		{30.0, false, "orange"},
		{80.0, false, "red"},
		{80.0, true, "muted"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := cpuColor(tt.cpu, tt.hidden)
			if tt.hidden && got != textMuted {
				t.Errorf("cpuColor(%f, true) should return textMuted", tt.cpu)
			}
			if !tt.hidden && got == textMuted {
				t.Errorf("cpuColor(%f, false) should not return textMuted", tt.cpu)
			}
			// Verify correct color thresholds
			if !tt.hidden {
				switch tt.want {
				case "gray":
					if got != cpuGray {
						t.Errorf("expected cpuGray for %.1f%%", tt.cpu)
					}
				case "green":
					if got != cpuGreen {
						t.Errorf("expected cpuGreen for %.1f%%", tt.cpu)
					}
				case "orange":
					if got != cpuOrange {
						t.Errorf("expected cpuOrange for %.1f%%", tt.cpu)
					}
				case "red":
					if got != cpuRed {
						t.Errorf("expected cpuRed for %.1f%%", tt.cpu)
					}
				}
			}
		})
	}
}

func TestSameDisplay(t *testing.T) {
	base := []proc.Info{{PID: 1, Name: "python: proj", CPU: 3.14159, RSS: 100 << 20, VMS: 1 << 30}}

	same := func(cur []proc.Info) bool { return sameDisplay(base, cur) }

	if !same(base) {
		t.Error("identical snapshots must be display-equal")
	}
	if !same([]proc.Info{{PID: 1, Name: "python: proj", CPU: 3.142, RSS: 100<<20 + 4096, VMS: 1 << 30}}) {
		t.Error("sub-display changes (CPU <0.1, RSS <1MB) must be display-equal")
	}
	if same([]proc.Info{{PID: 1, Name: "python: proj", CPU: 3.19, RSS: 100 << 20, VMS: 1 << 30}}) {
		t.Error("CPU change crossing the displayed decimal must differ")
	}
	if same([]proc.Info{{PID: 1, Name: "python: proj", CPU: 3.14159, RSS: 101 << 20, VMS: 1 << 30}}) {
		t.Error("RSS crossing a displayed MB must differ")
	}
	if same([]proc.Info{{PID: 1, Name: "other", CPU: 3.14159, RSS: 100 << 20, VMS: 1 << 30}}) {
		t.Error("name change must differ")
	}
	if same([]proc.Info{}) {
		t.Error("length change must differ")
	}
}
