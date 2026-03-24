// Package ui implements the Gio-based activity monitor interface.
package ui

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"sync"
	"syscall"
	"time"

	"activity/pkg/proc"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// SortColumn identifies which column is used for sorting.
type SortColumn int

const (
	// SortCPU sorts by CPU percentage descending.
	SortCPU SortColumn = iota
	// SortRSS sorts by physical memory descending.
	SortRSS
	// SortVMS sorts by virtual memory descending.
	SortVMS
	// SortName sorts by process name ascending.
	SortName
	// SortPID sorts by PID ascending.
	SortPID
)

// Column layout: name, pid, cpu, phys, virt
var colWidths = [5]unit.Dp{0, 90, 80, 100, 100} // name is flexed

// Design tokens
var (
	bgColor      = color.NRGBA{R: 0x0f, G: 0x0f, B: 0x0f, A: 0xff}
	surfaceColor = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	rowAltColor  = color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff}
	separatorClr = color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xff}
	headerBGClr  = color.NRGBA{R: 0x18, G: 0x18, B: 0x18, A: 0xff}

	textPrimary   = color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}
	textSecondary = color.NRGBA{R: 0xa8, G: 0xa8, B: 0xa8, A: 0xff}
	textMuted     = color.NRGBA{R: 0x72, G: 0x72, B: 0x72, A: 0xff}
	accentBlue    = color.NRGBA{R: 0x5c, G: 0x9c, B: 0xff, A: 0xff}

	cpuGray   = color.NRGBA{R: 0x5a, G: 0x5a, B: 0x5a, A: 0xff}
	cpuGreen  = color.NRGBA{R: 0x5c, G: 0xb8, B: 0x5c, A: 0xff}
	cpuOrange = color.NRGBA{R: 0xff, G: 0xb8, B: 0x4d, A: 0xff}
	cpuRed    = color.NRGBA{R: 0xff, G: 0x5c, B: 0x5c, A: 0xff}
)

// App holds the activity monitor application state.
type App struct {
	theme    *material.Theme
	win      *app.Window
	hideList *proc.HideList

	mu          sync.Mutex
	processes   []proc.Info
	showHidden  bool
	sortCol     SortColumn
	lastRefresh time.Time

	// persistent widget state
	list       widget.List
	toggleBtn  widget.Clickable
	headerName widget.Clickable
	headerPID  widget.Clickable
	headerCPU  widget.Clickable
	headerRSS  widget.Clickable
	headerVMS  widget.Clickable

	// right-click context menu
	rowTags []*bool // pointer event tags (stable pointers, one per visible row)
	menu    processMenu

	// table Y offset for absolute menu positioning
	tableOffsetY int
}

// NewApp creates a new activity monitor application.
func NewApp(win *app.Window, hideList *proc.HideList) *App {
	th := material.NewTheme()
	a := &App{
		theme:    th,
		win:      win,
		hideList: hideList,
		sortCol:  SortCPU,
	}
	a.list.Axis = layout.Vertical
	return a
}

// Refresh collects fresh process data. Call from a goroutine.
func (a *App) Refresh() {
	infos, err := proc.Collect()
	if err != nil {
		return
	}

	a.mu.Lock()
	a.processes = infos
	a.lastRefresh = time.Now()
	a.mu.Unlock()

	a.win.Invalidate()
}

// Layout renders the full UI frame.
func (a *App) Layout(gtx layout.Context) layout.Dimensions {
	// Fill background
	paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())

	a.mu.Lock()
	procs := a.processes
	showHidden := a.showHidden
	sortCol := a.sortCol
	a.mu.Unlock()

	a.handleHeaderClicks(gtx)

	if a.toggleBtn.Clicked(gtx) {
		a.mu.Lock()
		a.showHidden = !a.showHidden
		a.mu.Unlock()
	}

	visible := a.filterAndSort(procs, showHidden, sortCol)

	// Track cumulative Y for absolute menu positioning
	a.tableOffsetY = 0
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := a.layoutToolbar(gtx, len(procs), len(visible))
			a.tableOffsetY += d.Size.Y
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := a.layoutSeparator(gtx)
			a.tableOffsetY += d.Size.Y
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := a.layoutHeader(gtx, sortCol)
			a.tableOffsetY += d.Size.Y
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			d := a.layoutSeparator(gtx)
			a.tableOffsetY += d.Size.Y
			return d
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.layoutTable(gtx, visible)
		}),
	)

	// Context menu overlay (rendered last, on top of everything)
	result := a.menu.Layout(gtx, a.theme)
	if result.ok {
		switch result.action {
		case actionHide:
			a.hideList.Toggle(result.name)
		case actionInfo:
			go RunInfoWindow(result.pid, func(int32) {})
		case actionTree:
			go RunTreeWindow(result.pid, func(int32) {})
		case actionKill:
			syscall.Kill(int(result.pid), syscall.SIGKILL)
			go a.Refresh()
		}
	}

	return dims
}

func (a *App) handleHeaderClicks(gtx layout.Context) {
	type colBtn struct {
		btn *widget.Clickable
		col SortColumn
	}
	for _, cb := range []colBtn{
		{&a.headerName, SortName},
		{&a.headerPID, SortPID},
		{&a.headerCPU, SortCPU},
		{&a.headerRSS, SortRSS},
		{&a.headerVMS, SortVMS},
	} {
		if cb.btn.Clicked(gtx) {
			a.mu.Lock()
			a.sortCol = cb.col
			a.mu.Unlock()
		}
	}
}

func (a *App) filterAndSort(procs []proc.Info, showHidden bool, sortCol SortColumn) []proc.Info {
	visible := make([]proc.Info, 0, len(procs))
	for _, p := range procs {
		hidden := a.hideList.IsHidden(p.Name)
		if hidden && !showHidden {
			continue
		}
		visible = append(visible, p)
	}

	sort.Slice(visible, func(i, j int) bool {
		switch sortCol {
		case SortCPU:
			return visible[i].CPU > visible[j].CPU
		case SortRSS:
			return visible[i].RSS > visible[j].RSS
		case SortVMS:
			return visible[i].VMS > visible[j].VMS
		case SortName:
			return visible[i].Name < visible[j].Name
		case SortPID:
			return visible[i].PID < visible[j].PID
		default:
			return visible[i].CPU > visible[j].CPU
		}
	})

	return visible
}

func (a *App) layoutSeparator(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	w := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, separatorClr, clip.Rect{Max: image.Pt(w, h)}.Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (a *App) layoutToolbar(gtx layout.Context, total, visible int) layout.Dimensions {
	return layout.Inset{
		Top: unit.Dp(12), Bottom: unit.Dp(8),
		Left: unit.Dp(16), Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := fmt.Sprintf("%d processes", total)
				if total-visible > 0 {
					label += fmt.Sprintf("  (%d hidden)", total-visible)
				}
				l := material.Body2(a.theme, label)
				l.Color = textSecondary
				l.TextSize = unit.Sp(13)
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					var btnText string
					a.mu.Lock()
					if a.showHidden {
						btnText = "Hide filtered"
					} else {
						btnText = "Show all"
					}
					a.mu.Unlock()
					btn := material.Button(a.theme, &a.toggleBtn, btnText)
					btn.TextSize = unit.Sp(11)
					btn.Background = surfaceColor
					btn.Color = textSecondary
					btn.Inset = layout.Inset{
						Top: unit.Dp(4), Bottom: unit.Dp(4),
						Left: unit.Dp(12), Right: unit.Dp(12),
					}
					return btn.Layout(gtx)
				})
			}),
		)
	})
}

func (a *App) layoutHeader(gtx layout.Context, sortCol SortColumn) layout.Dimensions {
	headerH := gtx.Dp(unit.Dp(32))
	totalW := gtx.Constraints.Max.X

	paint.FillShape(gtx.Ops, headerBGClr, clip.Rect{Max: image.Pt(totalW, headerH)}.Op())

	type headerCol struct {
		label string
		btn   *widget.Clickable
		col   SortColumn
		align text.Alignment
	}
	cols := []headerCol{
		{"Process", &a.headerName, SortName, text.Start},
		{"PID", &a.headerPID, SortPID, text.End},
		{"CPU", &a.headerCPU, SortCPU, text.End},
		{"Memory", &a.headerRSS, SortRSS, text.End},
		{"Virtual", &a.headerVMS, SortVMS, text.End},
	}

	nameW := totalW
	for i := 1; i < len(colWidths); i++ {
		nameW -= gtx.Dp(colWidths[i])
	}

	x := 0
	for i, col := range cols {
		var colW int
		if i == 0 {
			colW = nameW
		} else {
			colW = gtx.Dp(colWidths[i])
		}

		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)

		gtxCol := gtx
		gtxCol.Constraints = layout.Exact(image.Pt(colW, headerH))

		material.Clickable(gtxCol, col.btn, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left: unit.Dp(16), Right: unit.Dp(16),
				Top: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := col.label
				if sortCol == col.col {
					label += " ^"
				}
				l := material.Body2(a.theme, label)
				l.Font.Weight = font.Medium
				l.TextSize = unit.Sp(11)
				l.Color = textMuted
				l.Alignment = col.align
				if sortCol == col.col {
					l.Color = accentBlue
				}
				return l.Layout(gtx)
			})
		})
		offset.Pop()
		x += colW
	}

	return layout.Dimensions{Size: image.Pt(totalW, headerH)}
}

func (a *App) layoutTable(gtx layout.Context, visible []proc.Info) layout.Dimensions {
	return material.List(a.theme, &a.list).Layout(gtx, len(visible), func(gtx layout.Context, index int) layout.Dimensions {
		p := visible[index]
		return a.layoutRow(gtx, p, index)
	})
}

func (a *App) layoutRow(gtx layout.Context, p proc.Info, index int) layout.Dimensions {
	rowH := gtx.Dp(unit.Dp(36))
	totalW := gtx.Constraints.Max.X
	hidden := a.hideList.IsHidden(p.Name)

	// Alternating row background
	if index%2 == 0 {
		paint.FillShape(gtx.Ops, rowAltColor, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
	}

	// Right-click detection per row (pointers are stable across appends)
	for len(a.rowTags) <= index {
		tag := new(bool)
		a.rowTags = append(a.rowTags, tag)
	}

	rowArea := clip.Rect{Max: image.Pt(totalW, rowH)}.Push(gtx.Ops)
	event.Op(gtx.Ops, a.rowTags[index])
	rowArea.Pop()

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: a.rowTags[index],
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			if e.Buttons.Contain(pointer.ButtonSecondary) {
				// e.Position is local to the row's clip area.
				// Compute absolute window position:
				// - tableOffsetY is the Y of the table top in window coords
				// - The row's Y within the visible viewport:
				//   (index - list.Position.First) * rowH - list.Position.Offset
				rowY := (index-a.list.Position.First)*rowH - a.list.Position.Offset
				absX := int(e.Position.X)
				absY := a.tableOffsetY + rowY + int(e.Position.Y)
				a.menu.Show(image.Pt(absX, absY), p.PID, p.Name)
			}
		}
	}

	nameW := totalW
	for i := 1; i < len(colWidths); i++ {
		nameW -= gtx.Dp(colWidths[i])
	}

	x := 0
	type colData struct {
		val   string
		width int
		align text.Alignment
		color color.NRGBA
		bold  bool
	}

	nameFG := textPrimary
	metricFG := textSecondary
	if hidden {
		nameFG = textMuted
		metricFG = textMuted
	}

	columns := []colData{
		{p.Name, nameW, text.Start, nameFG, true},
		{fmt.Sprintf("%d", p.PID), gtx.Dp(colWidths[1]), text.End, textMuted, false},
		{fmt.Sprintf("%.1f%%", p.CPU), gtx.Dp(colWidths[2]), text.End, cpuColor(p.CPU, hidden), false},
		{formatBytes(p.RSS), gtx.Dp(colWidths[3]), text.End, metricFG, false},
		{formatBytes(p.VMS), gtx.Dp(colWidths[4]), text.End, metricFG, false},
	}

	for _, col := range columns {
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)

		gtxCol := gtx
		gtxCol.Constraints = layout.Exact(image.Pt(col.width, rowH))

		layout.Inset{
			Left: unit.Dp(16), Right: unit.Dp(16),
			Top: unit.Dp(9),
		}.Layout(gtxCol, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, col.val)
			l.Color = col.color
			l.TextSize = unit.Sp(12)
			l.Alignment = col.align
			if col.bold {
				l.Font.Weight = font.Medium
			}
			l.MaxLines = 1
			return l.Layout(gtx)
		})

		offset.Pop()
		x += col.width
	}

	// Bottom separator
	sepOff := op.Offset(image.Pt(gtx.Dp(unit.Dp(16)), rowH-1)).Push(gtx.Ops)
	sepW := totalW - gtx.Dp(unit.Dp(32))
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff},
		clip.Rect{Max: image.Pt(sepW, 1)}.Op())
	sepOff.Pop()

	return layout.Dimensions{Size: image.Pt(totalW, rowH)}
}

// cpuColor returns a color based on CPU usage percentage.
func cpuColor(cpu float64, hidden bool) color.NRGBA {
	if hidden {
		return textMuted
	}
	switch {
	case cpu < 1:
		return cpuGray
	case cpu < 10:
		return cpuGreen
	case cpu < 50:
		return cpuOrange
	default:
		return cpuRed
	}
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
