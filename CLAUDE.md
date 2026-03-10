# Activity Monitor

macOS process monitor built with Go + Gio. Shows CPU%, physical memory, virtual memory with smart process naming (identifies which Python/Node/etc script is running).

## Project Structure

```
run                          # Shell facade (build/test/lint/check/run)
src/                         # Go module root
  cmd/activity/
    main.go                  # Entry point — event loop in goroutine, app.Main() on main
    icon_darwin.go           # Dock icon via NSApplication CGO
    gui/icon.png             # Transparent dock icon
  pkg/proc/
    proc.go                  # Process collection via gopsutil v4
    naming.go                # Smart naming: "python: project-dir" from cwd
    naming_test.go
    hide.go                  # Persistent hide list (JSON-backed)
    hide_test.go
  pkg/ui/
    app.go                   # Main UI layout, dark theme, color-coded CPU
    app_test.go
    window.go                # Window size persistence (debounced JSON save)
    window_darwin.go         # NSWindow frame autosave via CGO (position persistence)
local/                       # Gitignored — hidden.json, window.json
output/                      # Gitignored — bin/, testing/
```

## Key Commands

- `./run build` — Build to output/bin/activity
- `./run test [pkg]` — Run tests for a package
- `./run check` — Full quality gate (lint + all tests)
- `./run run` — Build and run
- `~/bin/activity` — Global wrapper

## Architecture

- **Gio v0.9.0** immediate-mode UI framework
- **gopsutil v4** for process info (CPU%, RSS, VMS, cmdline)
- **10s refresh interval** for gentle resource usage
- Managed by `auto` daemon (always running, crash recovery, login startup)

## Gotchas

- **Gio macOS daemon launch**: `app.Main()` MUST be on the main goroutine. Event loop goes in a goroutine. Without this, windows never appear when launched by auto/launchd.
- **Gio v0.9.0 API limitations**: No `app.Position`, no `pointer.InputOp` for per-row right-click. Use `font.Bold` not `text.Bold`.
- **Window position persistence**: Uses NSWindow frame autosave via CGO (`setFrameAutosaveName:`), not Gio APIs.
- **Column alignment**: Fixed dp widths for numeric columns (PID 90, CPU 80, Memory 100, Virtual 100), flexed name column.
- **Smart naming**: Always uses cwd project directory for interpreters (python, node, etc.) and cwd-labeled programs (claude). Script args are ignored — they're generic tools (uvicorn, flask, run) not project identifiers. Case-insensitive interpreter matching (macOS Homebrew uses capital-P "Python").
- **NSWindow autosave**: `setFrameAutosaveName:` needs nil guard — `stringWithUTF8String:` can return nil and crash without it.
