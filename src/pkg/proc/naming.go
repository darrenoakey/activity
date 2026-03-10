// Package proc provides process information collection with smart naming.
package proc

import (
	"path/filepath"
	"strings"
)

// interpreters maps binary base names to their language label.
var interpreters = map[string]string{
	"python":  "python",
	"python2": "python",
	"python3": "python",
	"node":    "node",
	"ruby":    "ruby",
	"perl":    "perl",
	"php":     "php",
	"lua":     "lua",
	"java":    "java",
	"bash":    "bash",
	"zsh":     "zsh",
	"sh":      "sh",
}

// SmartName derives a human-readable process name from the command line.
// For interpreted languages (Python, Node, etc.), it returns "lang: script_name".
// For regular binaries, it returns the binary base name.
// Falls back to the provided process name if cmdline is empty.
func SmartName(processName string, cmdline []string) string {
	if len(cmdline) == 0 {
		return processName
	}

	binary := filepath.Base(cmdline[0])

	lang, isInterpreter := interpreters[binary]
	if !isInterpreter {
		return binary
	}

	script := findScript(cmdline[1:])
	if script == "" {
		return binary
	}

	return lang + ": " + script
}

// findScript locates the script argument in a command line, skipping flags.
func findScript(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		// Skip common interpreter subcommands
		if arg == "run" || arg == "exec" || arg == "serve" {
			continue
		}
		return filepath.Base(arg)
	}
	return ""
}
