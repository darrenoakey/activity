package ui

import "testing"

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
