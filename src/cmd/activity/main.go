// Command activity is a macOS process monitor with smart naming and persistent filtering.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"activity/pkg/proc"
	"activity/pkg/ui"

	"gioui.org/app"
	"gioui.org/op"
)

func main() {
	// Resolve project root: executable lives in output/bin/ under project root
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(resolved)))
	localDir := filepath.Join(projectRoot, "local")
	hidePath := filepath.Join(localDir, "hidden.json")
	windowPath := filepath.Join(localDir, "window.json")

	hideList, err := proc.NewHideList(hidePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	windowState, err := ui.NewWindowPersist(windowPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	setDockIcon()

	// Run the event loop in a goroutine — app.Main() must own the main goroutine on macOS
	go func() {
		win := new(app.Window)
		win.Option(app.Title("Activity Monitor"))
		windowState.Apply(win)

		// Enable macOS native frame autosave for position persistence
		go func() {
			time.Sleep(500 * time.Millisecond)
			ui.EnableFrameAutosave("ActivityMonitor")
		}()

		monitor := ui.NewApp(win, hideList)

		go monitor.Refresh()

		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				monitor.Refresh()
			}
		}()

		var ops op.Ops
		for {
			switch e := win.Event().(type) {
			case app.DestroyEvent:
				if e.Err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", e.Err)
				}
				os.Exit(0)
			case app.ConfigEvent:
				c := e.Config
				windowState.UpdateSize(c.Size.X, c.Size.Y)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				monitor.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()

	// app.Main() runs the platform event loop on the main goroutine (required on macOS)
	app.Main()

	runtime.KeepAlive(hideList)
}
