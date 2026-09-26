package ui

import (
	"fmt"
	"image"
	"image/color"

	"activity/pkg/proc"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type headerCol struct {
	label string
	btn   *widget.Clickable
	col   SortColumn
	align text.Alignment
}

type colData struct {
	val   string
	width int
	align text.Alignment
	color color.NRGBA
	bold  bool
}

var trendWidths = []unit.Dp{0, 70, 90, 90, 80, 52}

func columnWidths(trends bool) []unit.Dp {
	if trends {
		return trendWidths
	}
	return colWidths[:]
}

func (a *App) headerSpec(trends bool) ([]headerCol, []unit.Dp) {
	if trends {
		return a.trendHeader(), columnWidths(true)
	}
	return a.liveHeader(), columnWidths(false)
}

func (a *App) liveHeader() []headerCol {
	return []headerCol{
		{"Process", &a.headerName, SortName, text.Start},
		{"PID", &a.headerPID, SortPID, text.End},
		{"CPU", &a.headerCPU, SortCPU, text.End},
		{"Memory", &a.headerRSS, SortRSS, text.End},
		{"Virtual", &a.headerVMS, SortVMS, text.End},
	}
}

func (a *App) trendHeader() []headerCol {
	return []headerCol{
		{"Process", &a.headerName, SortName, text.Start},
		{"PID", &a.headerPID, SortPID, text.End},
		{"Avg mem", &a.headerAvgRSS, SortAvgRSS, text.End},
		{"Memory", &a.headerRSS, SortRSS, text.End},
		{"Avg CPU", &a.headerAvgCPU, SortAvgCPU, text.End},
		{"N", &a.headerSamples, SortSamples, text.End},
	}
}

func rowColumns(gtx layout.Context, p proc.Info, widths []unit.Dp, nameW int, nameFG, metricFG color.NRGBA, hidden, trends bool) []colData {
	if trends {
		return trendColumns(gtx, p, widths, nameW, nameFG, metricFG)
	}
	return liveColumns(gtx, p, widths, nameW, nameFG, metricFG, hidden)
}

func liveColumns(gtx layout.Context, p proc.Info, widths []unit.Dp, nameW int, nameFG, metricFG color.NRGBA, hidden bool) []colData {
	return []colData{
		{p.Name, nameW, text.Start, nameFG, true},
		{fmt.Sprintf("%d", p.PID), gtx.Dp(widths[1]), text.End, textMuted, false},
		{fmt.Sprintf("%.1f%%", p.CPU), gtx.Dp(widths[2]), text.End, cpuColor(p.CPU, hidden), false},
		{formatBytes(p.RSS), gtx.Dp(widths[3]), text.End, metricFG, false},
		{formatBytes(p.VMS), gtx.Dp(widths[4]), text.End, metricFG, false},
	}
}

func trendColumns(gtx layout.Context, p proc.Info, widths []unit.Dp, nameW int, nameFG, metricFG color.NRGBA) []colData {
	return []colData{
		{p.Name, nameW, text.Start, nameFG, true},
		{fmt.Sprintf("%d", p.PID), gtx.Dp(widths[1]), text.End, textMuted, false},
		{formatBytes(p.AverageRSS()), gtx.Dp(widths[2]), text.End, metricFG, false},
		{formatBytes(p.RSS), gtx.Dp(widths[3]), text.End, metricFG, false},
		{fmt.Sprintf("%.1f%%", p.AverageCPU()), gtx.Dp(widths[4]), text.End, metricFG, false},
		{fmt.Sprintf("%d", p.Samples), gtx.Dp(widths[5]), text.End, textMuted, false},
	}
}

func (a *App) layoutAnalysis(gtx layout.Context, busy bool, errText string, lines []string) layout.Dimensions {
	if !busy && errText == "" && len(lines) == 0 {
		return layout.Dimensions{}
	}
	h := gtx.Dp(unit.Dp(200))
	w := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, surfaceColor, clip.Rect{Max: image.Pt(w, h)}.Op())
	gtx.Constraints = layout.Exact(image.Pt(w, h))
	layout.Inset{
		Top: unit.Dp(8), Bottom: unit.Dp(8),
		Left: unit.Dp(16), Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutAnalysisHead(gtx, busy)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.layoutAnalysisBody(gtx, errText, lines)
			}),
		)
	})
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func (a *App) layoutAnalysisHead(gtx layout.Context, busy bool) layout.Dimensions {
	title := "Suggestions"
	if busy {
		title = "Asking agentic-high…"
	}
	return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, title)
			l.Color = accentBlue
			l.TextSize = unit.Sp(12)
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutToolButton(gtx, &a.dismissBtn, "Dismiss", false)
		}),
	)
}

func (a *App) layoutAnalysisBody(gtx layout.Context, errText string, lines []string) layout.Dimensions {
	if errText != "" {
		l := material.Body2(a.theme, errText)
		l.Color = cpuRed
		l.TextSize = unit.Sp(12)
		return l.Layout(gtx)
	}
	return material.List(a.theme, &a.analyseList).Layout(gtx, len(lines), func(gtx layout.Context, index int) layout.Dimensions {
		l := material.Body2(a.theme, lines[index])
		l.Color = textPrimary
		l.TextSize = unit.Sp(12)
		return l.Layout(gtx)
	})
}
