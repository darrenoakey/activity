package ui

import (
	"fmt"
	"image"
	"image/color"
	"sort"

	"activity/pkg/proc"

	giomenu "github.com/darrenoakey/daz-golang-gio/menu"

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

// flatRow is a tree node flattened for display with an indentation level.
type flatRow struct {
	node   *proc.TreeNode
	indent int
	prefix string // tree connector prefix like "├─ " or "└─ "
}

// RunTreeWindow opens a new window showing the process tree for the given PID.
// Runs its own event loop — call from a goroutine.
func RunTreeWindow(pid int32, onClose func(int32)) {
	defer onClose(pid)

	root, err := proc.BuildTree(pid)

	win := new(app.Window)
	title := fmt.Sprintf("Tree: PID %d", pid)
	win.Option(
		app.Title(title),
		app.Size(unit.Dp(600), unit.Dp(500)),
	)

	th := material.NewTheme()

	var (
		list    widget.List
		rows    []flatRow
		rowTags []bool // pointer event tags per row
		menu    processMenu
		errMsg  string
	)
	list.Axis = layout.Vertical

	if err != nil {
		errMsg = fmt.Sprintf("Error building tree: %v", err)
	} else {
		rows = flattenTree(root, 0, "")
		rowTags = make([]bool, len(rows))
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
				layoutTreeContent(gtx, th, &list, rows, rowTags, &menu)
			}
			e.Frame(gtx.Ops)
		}
	}
}

func flattenTree(node *proc.TreeNode, depth int, prefix string) []flatRow {
	if node == nil {
		return nil
	}

	rows := []flatRow{{node: node, indent: depth, prefix: prefix}}

	// Sort children by PID for stable display
	children := make([]*proc.TreeNode, len(node.Children))
	copy(children, node.Children)
	sort.Slice(children, func(i, j int) bool {
		return children[i].PID < children[j].PID
	})

	for i, child := range children {
		var childPrefix string
		if i == len(children)-1 {
			childPrefix = "└─ "
		} else {
			childPrefix = "├─ "
		}
		rows = append(rows, flattenTree(child, depth+1, childPrefix)...)
	}

	return rows
}

var (
	targetHighlight = color.NRGBA{R: 0x1a, G: 0x2a, B: 0x4a, A: 0xff}
	connectorColor  = color.NRGBA{R: 0x4a, G: 0x4a, B: 0x4a, A: 0xff}
)

func layoutTreeContent(gtx layout.Context, th *material.Theme, list *widget.List,
	rows []flatRow, rowTags []bool, menu *processMenu) layout.Dimensions {

	dims := material.List(th, list).Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
		row := rows[index]
		return layoutTreeRow(gtx, th, row, index, rowTags, menu)
	})

	// Render context menu overlay
	result := menu.Layout(gtx, th)
	if result.ok {
		if result.action == actionInfo {
			go RunInfoWindow(result.pid, func(p int32) {})
		}
	}

	return dims
}

func layoutTreeRow(gtx layout.Context, th *material.Theme, row flatRow, index int, rowTags []bool, menu *processMenu) layout.Dimensions {
	rowH := gtx.Dp(unit.Dp(30))
	totalW := gtx.Constraints.Max.X

	// Highlight target process
	if row.node.IsTarget {
		paint.FillShape(gtx.Ops, targetHighlight, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
	} else if index%2 == 0 {
		paint.FillShape(gtx.Ops, rowAltColor, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
	}

	// Right-click detection
	area := clip.Rect{Max: image.Pt(totalW, rowH)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &rowTags[index])
	area.Pop()

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &rowTags[index],
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			if e.Buttons.Contain(pointer.ButtonSecondary) {
				// Show menu with only Info option for tree rows
				infoOnly := []giomenu.Item{{Label: "Info"}}
				menu.ShowItems(row.node.PID, row.node.Name, infoOnly)
			}
		}
	}

	// Indentation
	indentPx := gtx.Dp(unit.Dp(20)) * row.indent
	x := gtx.Dp(unit.Dp(16)) + indentPx

	// Tree connector prefix
	if row.prefix != "" {
		prefixOff := op.Offset(image.Pt(x-gtx.Dp(unit.Dp(20)), gtx.Dp(unit.Dp(6)))).Push(gtx.Ops)
		gtxPfx := gtx
		gtxPfx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(20)), rowH))
		l := material.Body2(th, row.prefix)
		l.Color = connectorColor
		l.TextSize = unit.Sp(11)
		l.Layout(gtxPfx)
		prefixOff.Pop()
	}

	// Process label: "name (pid)"
	label := fmt.Sprintf("%s (%d)", row.node.Name, row.node.PID)
	labelOff := op.Offset(image.Pt(x, gtx.Dp(unit.Dp(6)))).Push(gtx.Ops)
	gtxLabel := gtx
	gtxLabel.Constraints = layout.Exact(image.Pt(totalW-x-gtx.Dp(unit.Dp(16)), rowH-gtx.Dp(unit.Dp(12))))

	l := material.Body2(th, label)
	l.TextSize = unit.Sp(12)
	l.MaxLines = 1
	l.Alignment = text.Start
	if row.node.IsTarget {
		l.Color = accentBlue
		l.Font.Weight = font.Bold
	} else {
		l.Color = textPrimary
		l.Font.Weight = font.Normal
	}
	l.Layout(gtxLabel)
	labelOff.Pop()

	// Bottom separator
	sepOff := op.Offset(image.Pt(gtx.Dp(unit.Dp(16)), rowH-1)).Push(gtx.Ops)
	sepW := totalW - gtx.Dp(unit.Dp(32))
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff},
		clip.Rect{Max: image.Pt(sepW, 1)}.Op())
	sepOff.Pop()

	return layout.Dimensions{Size: image.Pt(totalW, rowH)}
}
