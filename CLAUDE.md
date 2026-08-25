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
    proc.go                  # Info type + ExtendedInfo via gopsutil v4 (on-demand)
    monitor.go               # Cached low-cost sampler: identity once per PID, 1 syscall/pid
    naming.go                # Smart naming: "python: project-dir" from cwd
    naming_test.go
    monitor_test.go
    tree.go                  # Process tree building (ancestor chain + full subtree)
    hide.go                  # Persistent hide list (JSON-backed)
    hide_test.go
  pkg/ui/
    app.go                   # Main UI layout, dark theme, color-coded CPU, right-click menu
    app_test.go
    menu.go                  # Process menu wrapper over daz-golang-gio/menu (Hide/Tree/Info/Kill)
    info.go                  # Process info window (details + environment tabs)
    tree.go                  # Process tree window (ancestor lineage + subtree)
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

- **Gio v0.9.0** immediate-mode UI framework; **daz-golang-gio** for window persistence (`persist`) and context menus (`menu`)
- **gopsutil v4** only for one-shot lookups (info window); never in a sampling loop — see gotcha below
- **proc.Monitor** sampling loop: identity cached once per PID lifetime (start-time keyed), one `kern.proc.all` sysctl + one `proc_pidinfo(PROC_PIDTASKINFO)` syscall per process via a single persistent purego libproc handle. 5s cadence; CPU is per-window delta (Activity Monitor semantics), lifetime-average on first sight
- **Render on change**: `App.Refresh` repaints only when displayed text (pid, name, CPU at 0.1, memory at formatBytes granularity) differs — an idle machine renders zero frames. `sameDisplay`/`rowDisplayKey` must stay in sync with `layoutRow` formatting and `formatBytes` thresholds
- Managed by launchd (`com.darrenoakey.activity`), not auto; restart with `launchctl kickstart -k gui/$(id -u)/com.darrenoakey.activity`

## Gotchas

- **Gio macOS daemon launch**: `app.Main()` MUST be on the main goroutine. Event loop goes in a goroutine. Without this, windows never appear when launched by auto/launchd.
- **Gio v0.9.0 API limitations**: No `app.Position`. Use `font.Bold` not `text.Bold`.
- **Per-row right-click**: Works via `pointer.Filter{Target: tag, Kinds: pointer.Press}` + `event.Op(ops, tag)` within a `clip.Rect`. Check `e.Buttons.Contain(pointer.ButtonSecondary)`. Use stable pointer tags (`[]*bool` not `[]bool`) to survive slice reallocation.
- **Gio pointer event stale events**: When hiding a handler (e.g. context menu dismissed), always drain queued events for its tags on the next frame, or they'll fire when the handler is next shown.
- **Multi-window**: Create new `app.Window` in goroutines with their own event loops. `app.Main()` stays on the main goroutine. Each window needs its own `material.Theme` instance.
- **Process tree pruning**: `BuildTree()` prunes ancestors to only show the single child on the path to the target. The target's full subtree is preserved.
- **Window position persistence**: Uses NSWindow frame autosave via CGO (`setFrameAutosaveName:`), not Gio APIs.
- **Column alignment**: Fixed dp widths for numeric columns (PID 90, CPU 80, Memory 100, Virtual 100), flexed name column.
- **Smart naming**: Always uses cwd project directory for interpreters (python, node, etc.) and cwd-labeled programs (claude). Script args are ignored — they're generic tools (uvicorn, flask, run) not project identifiers. Case-insensitive interpreter matching (macOS Homebrew uses capital-P "Python").
- **NSWindow autosave**: `setFrameAutosaveName:` needs nil guard — `stringWithUTF8String:` can return nil and crash without it.
- **gopsutil v4 darwin dlopens per call**: every `Times()`, `MemoryInfo()`, `Cwd()` call does `purego.Dlopen(libSystem)` + dlclose (v4.26.x `loadProcFuncs`). In a per-process sampling loop that's thousands of dlopens per refresh — 71ms for ~1000 procs. That's why `proc.Monitor` keeps its own persistent libproc handle and one-shot lookups are the only sanctioned gopsutil use.
- **Phantom CPU for wedged tasks**: XNU can bill a task user-time at wall rate for a permanently-runnable but never-scheduled thread (seen: Activity Monitor.app wedged after 2.5 days — `pti_total_user` advanced 5s/5s while `ps` TIME stayed frozen). Every tick-based monitor (ours, Apple `top`) shows such a task at ~100%; `ps %CPU`/`TIME` (BSD accounting) does not. Fix the process (restart it), not the monitor.
