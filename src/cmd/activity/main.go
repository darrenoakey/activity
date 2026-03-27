// Command activity is a macOS process monitor with smart naming and persistent filtering.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"activity/pkg/proc"
	"activity/pkg/ui"

	"gioui.org/app"
	"gioui.org/op"
	"github.com/darrenoakey/daz-golang-gio/macos"
	"github.com/darrenoakey/daz-golang-gio/persist"
)

//go:embed gui/icon.png
var dockIconBytes []byte

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

	hideList, err := proc.NewHideList(hidePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Run the event loop in a goroutine — app.Main() must own the main goroutine on macOS
	go func() {
		w := persist.NewWindow("activity", app.Title("Activity Monitor"))

		monitor := ui.NewApp(w.Window, hideList)

		go monitor.Refresh()

		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				monitor.Refresh()
			}
		}()

		var ops op.Ops
		var iconOnce sync.Once
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				if e.Err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", e.Err)
				}
				w.Close()
				os.Exit(0)
			case app.FrameEvent:
				iconOnce.Do(func() { macos.SetDockIcon(dockIconBytes) })
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
