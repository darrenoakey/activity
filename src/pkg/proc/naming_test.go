package proc

import "testing"

func TestSmartName(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		cmdline     []string
		cwd         string
		want        string
	}{
		{
			name:        "empty cmdline falls back to process name",
			processName: "Mystery",
			cmdline:     nil,
			want:        "Mystery",
		},
		{
			name:        "regular binary uses binary name",
			processName: "nginx",
			cmdline:     []string{"/usr/sbin/nginx", "-g", "daemon off;"},
			want:        "nginx",
		},
		{
			name:        "custom python binary name preserved",
			processName: "Python",
			cmdline:     []string{"auto-blog"},
			cwd:         "/Volumes/T9/src/auto-blog",
			want:        "auto-blog",
		},

		// Python — always uses cwd project name
		{
			name:        "python with script uses cwd not script",
			processName: "Python",
			cmdline:     []string{"/usr/bin/python3", "server.py"},
			cwd:         "/Volumes/T9/src/my-project",
			want:        "python: my-project",
		},
		{
			name:        "homebrew Python uses cwd",
			processName: "Python",
			cmdline:     []string{"/opt/homebrew/Cellar/python@3.14/3.14.3_1/Frameworks/Python.framework/Versions/3.14/Resources/Python.app/Contents/MacOS/Python"},
			cwd:         "/Volumes/T9/src/theassistant",
			want:        "python: theassistant",
		},
		{
			name:        "python uvicorn uses cwd",
			processName: "Python",
			cmdline:     []string{"python3", "-m", "uvicorn", "src.server:app"},
			cwd:         "/Volumes/T9/src/chatty",
			want:        "python: chatty",
		},
		{
			name:        "python with run arg uses cwd",
			processName: "Python",
			cmdline:     []string{"/usr/bin/python3", "run"},
			cwd:         "/Volumes/T9/src/beezle",
			want:        "python: beezle",
		},
		{
			name:        "python with no cwd falls back to binary",
			processName: "Python",
			cmdline:     []string{"python3"},
			cwd:         "",
			want:        "python3",
		},
		{
			name:        "python with root cwd falls back to binary",
			processName: "Python",
			cmdline:     []string{"/opt/homebrew/Cellar/python@3.14/3.14.3_1/Frameworks/Python.framework/Versions/3.14/Resources/Python.app/Contents/MacOS/Python"},
			cwd:         "/",
			want:        "Python",
		},

		// Node — same cwd strategy
		{
			name:        "node uses cwd",
			processName: "node",
			cmdline:     []string{"/usr/local/bin/node", "server.js"},
			cwd:         "/app/my-service",
			want:        "node: my-service",
		},

		// Claude — cwd-labeled program
		{
			name:        "claude with project cwd",
			processName: "claude",
			cmdline:     []string{"claude"},
			cwd:         "/Volumes/T9/src/activity",
			want:        "claude: activity",
		},
		{
			name:        "claude without cwd",
			processName: "claude",
			cmdline:     []string{"claude"},
			cwd:         "",
			want:        "claude",
		},
		{
			name:        "claude desktop app with root cwd",
			processName: "Claude",
			cmdline:     []string{"/Applications/Claude.app/Contents/MacOS/Claude"},
			cwd:         "/",
			want:        "Claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SmartName(tt.processName, tt.cmdline, tt.cwd)
			if got != tt.want {
				t.Errorf("SmartName(%q, %v, %q) = %q, want %q", tt.processName, tt.cmdline, tt.cwd, got, tt.want)
			}
		})
	}
}
