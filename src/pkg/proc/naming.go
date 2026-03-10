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

// cwdPrograms maps binary names that should be labeled by their working directory.
// These programs have no useful distinguishing info in their cmdline args.
var cwdPrograms = map[string]bool{
	"claude": true,
}

// SmartName derives a human-readable process name from the command line and cwd.
//
// Naming strategy (in priority order):
//  1. If cmdline is empty, use the OS process name.
//  2. If the binary is a cwd-labeled program (e.g. claude), use "name: dir".
//  3. If the binary is an interpreter (python, node, etc.), use "lang: dir" from cwd.
//     Script args are almost always generic tools (uvicorn, flask, run, pyright-langserver)
//     or -c garbage — the cwd project directory is what actually identifies the process.
//  4. Otherwise, use the binary base name.
func SmartName(processName string, cmdline []string, cwd string) string {
	if len(cmdline) == 0 {
		return processName
	}

	binary := filepath.Base(cmdline[0])
	binaryLower := strings.ToLower(binary)

	if cwdPrograms[binaryLower] {
		if cwd != "" && cwd != "/" {
			return binaryLower + ": " + filepath.Base(cwd)
		}
		return binary
	}

	lang, isInterpreter := interpreters[binaryLower]
	if !isInterpreter {
		return binary
	}

	if cwd != "" && cwd != "/" {
		return lang + ": " + filepath.Base(cwd)
	}

	return binary
}
