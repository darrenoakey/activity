package ui

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"
	"time"

	"activity/pkg/proc"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// RunInfoWindow opens a new window showing detailed process information.
// Runs its own event loop — call from a goroutine.
func RunInfoWindow(pid int32, onClose func(int32)) {
	defer onClose(pid)

	info, err := proc.FetchExtended(pid)

	win := new(app.Window)
	title := fmt.Sprintf("Info: PID %d", pid)
	if err == nil {
		title = fmt.Sprintf("Info: %s (%d)", info.Name, pid)
	}
	win.Option(
		app.Title(title),
		app.Size(unit.Dp(520), unit.Dp(650)),
	)

	th := material.NewTheme()

	var (
		procList  widget.List
		envList   widget.List
		tabProc   widget.Clickable
		tabEnv    widget.Clickable
		activeTab int // 0=process, 1=environment
	)
	procList.Axis = layout.Vertical
	envList.Axis = layout.Vertical

	// Build display data
	var procRows []kvRow
	var envRows []kvRow
	var errMsg string

	if err != nil {
		errMsg = fmt.Sprintf("Error: %v", err)
	} else {
		procRows = buildProcRows(info)
		envRows = buildEnvRows(info.Environ)
	}

	var ops op.Ops
	for {
		switch e := win.Event().(type) {
		case app.DestroyEvent:
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())

			if errMsg != "" {
				layoutError(gtx, th, errMsg)
			} else {
				layoutInfoContent(gtx, th, &procList, &envList, &tabProc, &tabEnv, &activeTab, procRows, envRows)
			}
			e.Frame(gtx.Ops)
		}
	}
}

type kvRow struct {
	key   string
	value string
}

func buildProcRows(info proc.ExtendedInfo) []kvRow {
	startTime := ""
	if info.CreateTime > 0 {
		t := time.UnixMilli(info.CreateTime)
		startTime = t.Format("2006-01-02 15:04:05")
	}

	rows := []kvRow{
		{"Name", info.Name},
		{"PID", fmt.Sprintf("%d", info.PID)},
		{"Raw Name", info.RawName},
		{"Command", info.Cmdline},
		{"Working Dir", info.Cwd},
		{"CPU", fmt.Sprintf("%.1f%%", info.CPU)},
		{"Memory (RSS)", formatBytes(info.RSS)},
		{"Virtual (VMS)", formatBytes(info.VMS)},
		{"Status", info.Status},
		{"User", info.Username},
		{"Threads", fmt.Sprintf("%d", info.NumThreads)},
		{"Started", startTime},
		{"Parent PID", fmt.Sprintf("%d", info.Ppid)},
	}
	return rows
}

func buildEnvRows(environ []string) []kvRow {
	sort.Strings(environ)
	rows := make([]kvRow, 0, len(environ))
	for _, env := range environ {
		k, v, _ := strings.Cut(env, "=")
		rows = append(rows, kvRow{key: k, value: v})
	}
	return rows
}

var (
	tabActiveBG   = color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xff}
	tabInactiveBG = color.NRGBA{R: 0x18, G: 0x18, B: 0x18, A: 0xff}
)

func layoutInfoContent(gtx layout.Context, th *material.Theme, procList, envList *widget.List,
	tabProc, tabEnv *widget.Clickable, activeTab *int, procRows, envRows []kvRow) layout.Dimensions {

	if tabProc.Clicked(gtx) {
		*activeTab = 0
	}
	if tabEnv.Clicked(gtx) {
		*activeTab = 1
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutInfoTabs(gtx, th, tabProc, tabEnv, *activeTab)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			h := gtx.Dp(unit.Dp(1))
			w := gtx.Constraints.Max.X
			paint.FillShape(gtx.Ops, separatorClr, clip.Rect{Max: image.Pt(w, h)}.Op())
			return layout.Dimensions{Size: image.Pt(w, h)}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if *activeTab == 0 {
				return layoutProcTab(gtx, th, procList, procRows)
			}
			return layoutEnvTab(gtx, th, envList, envRows)
		}),
	)
}

func layoutInfoTabs(gtx layout.Context, th *material.Theme, tabProc, tabEnv *widget.Clickable, activeTab int) layout.Dimensions {
	tabH := gtx.Dp(unit.Dp(36))
	totalW := gtx.Constraints.Max.X

	paint.FillShape(gtx.Ops, headerBGClr, clip.Rect{Max: image.Pt(totalW, tabH)}.Op())

	type tab struct {
		label  string
		btn    *widget.Clickable
		active bool
	}
	tabs := []tab{
		{"Process", tabProc, activeTab == 0},
		{"Environment", tabEnv, activeTab == 1},
	}

	x := 0
	tabW := totalW / len(tabs)
	for _, t := range tabs {
		off := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)

		bg := tabInactiveBG
		fg := textMuted
		if t.active {
			bg = tabActiveBG
			fg = accentBlue
		}

		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: image.Pt(tabW, tabH)}.Op())

		gtxTab := gtx
		gtxTab.Constraints = layout.Exact(image.Pt(tabW, tabH))

		material.Clickable(gtxTab, t.btn, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(th, t.label)
				l.Color = fg
				l.TextSize = unit.Sp(13)
				l.Font.Weight = font.Medium
				l.Alignment = text.Middle
				return l.Layout(gtx)
			})
		})

		off.Pop()
		x += tabW
	}

	return layout.Dimensions{Size: image.Pt(totalW, tabH)}
}

func layoutProcTab(gtx layout.Context, th *material.Theme, list *widget.List, rows []kvRow) layout.Dimensions {
	return material.List(th, list).Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
		row := rows[index]
		rowH := gtx.Dp(unit.Dp(40))
		totalW := gtx.Constraints.Max.X

		if index%2 == 0 {
			paint.FillShape(gtx.Ops, rowAltColor, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
		}

		// Key label
		keyW := gtx.Dp(unit.Dp(110))
		keyOff := op.Offset(image.Pt(gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(11)))).Push(gtx.Ops)
		gtxKey := gtx
		gtxKey.Constraints = layout.Exact(image.Pt(keyW, rowH-gtx.Dp(unit.Dp(22))))
		l := material.Body2(th, row.key)
		l.Color = textMuted
		l.TextSize = unit.Sp(12)
		l.Font.Weight = font.Medium
		l.Layout(gtxKey)
		keyOff.Pop()

		// Value
		valX := gtx.Dp(unit.Dp(130))
		valW := totalW - valX - gtx.Dp(unit.Dp(16))
		valOff := op.Offset(image.Pt(valX, gtx.Dp(unit.Dp(11)))).Push(gtx.Ops)
		gtxVal := gtx
		gtxVal.Constraints = layout.Exact(image.Pt(valW, rowH-gtx.Dp(unit.Dp(22))))
		v := material.Body2(th, row.value)
		v.Color = textPrimary
		v.TextSize = unit.Sp(12)
		v.MaxLines = 1
		v.Layout(gtxVal)
		valOff.Pop()

		// Separator
		sepOff := op.Offset(image.Pt(gtx.Dp(unit.Dp(16)), rowH-1)).Push(gtx.Ops)
		sepW := totalW - gtx.Dp(unit.Dp(32))
		paint.FillShape(gtx.Ops, color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff},
			clip.Rect{Max: image.Pt(sepW, 1)}.Op())
		sepOff.Pop()

		return layout.Dimensions{Size: image.Pt(totalW, rowH)}
	})
}

func layoutEnvTab(gtx layout.Context, th *material.Theme, list *widget.List, rows []kvRow) layout.Dimensions {
	if len(rows) == 0 {
		return layout.Inset{Top: unit.Dp(20), Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(th, "No environment variables available (permission denied)")
			l.Color = textMuted
			l.TextSize = unit.Sp(12)
			return l.Layout(gtx)
		})
	}

	return material.List(th, list).Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
		row := rows[index]
		rowH := gtx.Dp(unit.Dp(32))
		totalW := gtx.Constraints.Max.X

		if index%2 == 0 {
			paint.FillShape(gtx.Ops, rowAltColor, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
		}

		// Key
		keyOff := op.Offset(image.Pt(gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(7)))).Push(gtx.Ops)
		keyW := gtx.Dp(unit.Dp(180))
		gtxKey := gtx
		gtxKey.Constraints = layout.Exact(image.Pt(keyW, rowH-gtx.Dp(unit.Dp(14))))
		l := material.Body2(th, row.key)
		l.Color = accentBlue
		l.TextSize = unit.Sp(11)
		l.Font.Weight = font.Medium
		l.MaxLines = 1
		l.Layout(gtxKey)
		keyOff.Pop()

		// Value
		valX := gtx.Dp(unit.Dp(200))
		valW := totalW - valX - gtx.Dp(unit.Dp(16))
		valOff := op.Offset(image.Pt(valX, gtx.Dp(unit.Dp(7)))).Push(gtx.Ops)
		gtxVal := gtx
		gtxVal.Constraints = layout.Exact(image.Pt(valW, rowH-gtx.Dp(unit.Dp(14))))
		v := material.Body2(th, row.value)
		v.Color = textSecondary
		v.TextSize = unit.Sp(11)
		v.MaxLines = 1
		v.Layout(gtxVal)
		valOff.Pop()

		return layout.Dimensions{Size: image.Pt(totalW, rowH)}
	})
}

func layoutError(gtx layout.Context, th *material.Theme, msg string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(20), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Body1(th, msg)
		l.Color = cpuRed
		return l.Layout(gtx)
	})
}
