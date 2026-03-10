package proc

import "testing"

func TestSmartName(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		cmdline     []string
		want        string
	}{
		{
			name:        "python with script",
			processName: "Python",
			cmdline:     []string{"/usr/bin/python3", "/home/user/my_server.py", "--port", "8080"},
			want:        "python: my_server.py",
		},
		{
			name:        "python with flags before script",
			processName: "Python",
			cmdline:     []string{"python3", "-u", "-m", "my_module"},
			want:        "python: my_module",
		},
		{
			name:        "node with script",
			processName: "node",
			cmdline:     []string{"/usr/local/bin/node", "/app/server.js"},
			want:        "node: server.js",
		},
		{
			name:        "regular binary",
			processName: "nginx",
			cmdline:     []string{"/usr/sbin/nginx", "-g", "daemon off;"},
			want:        "nginx",
		},
		{
			name:        "empty cmdline falls back to process name",
			processName: "Mystery",
			cmdline:     nil,
			want:        "Mystery",
		},
		{
			name:        "python with no script",
			processName: "Python",
			cmdline:     []string{"python3"},
			want:        "python3",
		},
		{
			name:        "ruby with script",
			processName: "ruby",
			cmdline:     []string{"/usr/bin/ruby", "sidekiq.rb"},
			want:        "ruby: sidekiq.rb",
		},
		{
			name:        "bash script",
			processName: "bash",
			cmdline:     []string{"/bin/bash", "/usr/local/bin/my-daemon.sh"},
			want:        "bash: my-daemon.sh",
		},
		{
			name:        "java with jar",
			processName: "java",
			cmdline:     []string{"java", "-Xmx2g", "-jar", "app.jar"},
			want:        "java: app.jar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SmartName(tt.processName, tt.cmdline)
			if got != tt.want {
				t.Errorf("SmartName(%q, %v) = %q, want %q", tt.processName, tt.cmdline, got, tt.want)
			}
		})
	}
}
