package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// MenuAction identifies a context menu action.
type MenuAction int

const (
	ActionHide MenuAction = iota
	ActionTree
	ActionInfo
	ActionKill
)

// MenuItem defines a single context menu entry.
type MenuItem struct {
	Label  string
	Action MenuAction
}

// DefaultMenuItems is the standard set of context menu actions.
var DefaultMenuItems = []MenuItem{
	{"Hide", ActionHide},
	{"Tree", ActionTree},
	{"Info", ActionInfo},
	{"Kill", ActionKill},
}

// ContextMenu is a reusable floating context menu component with hover highlighting.
type ContextMenu struct {
	visible    bool
	showFrame  bool        // skip bg dismiss on the frame Show() was called
	pos        image.Point // absolute window position
	targetPID  int32
	targetName string
	items      []MenuItem

	itemTags []*bool // stable pointer event tags, one per item
	bgTag    bool    // background dismiss tag

	hoverIdx int // currently hovered item index, -1 = none
}

// menuResult is returned from Layout when an action is triggered.
type menuResult struct {
	action MenuAction
	pid    int32
	name   string
	ok     bool
}

const (
	menuItemH  = 32
	menuW      = 120
	menuPadTop = 4
	menuPadBot = 4
)

var (
	menuBG      = color.NRGBA{R: 0x28, G: 0x28, B: 0x28, A: 0xf0}
	menuBorder  = color.NRGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff}
	menuHoverBG = color.NRGBA{R: 0x24, G: 0x3e, B: 0x6c, A: 0xff}
	menuKillFG  = color.NRGBA{R: 0xff, G: 0x5c, B: 0x5c, A: 0xff}
)

// Show opens the context menu at the given window position for the given process.
func (m *ContextMenu) Show(pos image.Point, pid int32, name string) {
	m.visible = true
	m.showFrame = true
	m.pos = pos
	m.targetPID = pid
	m.targetName = name
	m.items = DefaultMenuItems
	m.hoverIdx = -1
	m.ensureTags()
}

// Dismiss closes the context menu.
func (m *ContextMenu) Dismiss() {
	m.visible = false
	m.hoverIdx = -1
}

// Visible returns whether the context menu is currently shown.
func (m *ContextMenu) Visible() bool {
	return m.visible
}

// ensureTags allocates stable pointer tags for each item.
func (m *ContextMenu) ensureTags() {
	for len(m.itemTags) < len(m.items) {
		tag := new(bool)
		m.itemTags = append(m.itemTags, tag)
	}
}

// clampPosition adjusts a menu position to stay within window bounds.
func clampPosition(pos image.Point, menuW, menuH, winW, winH int) image.Point {
	if maxX := winW - menuW; pos.X > maxX {
		pos.X = maxX
	}
	if maxY := winH - menuH; pos.Y > maxY {
		pos.Y = maxY
	}
	if pos.X < 0 {
		pos.X = 0
	}
	if pos.Y < 0 {
		pos.Y = 0
	}
	return pos
}

// drainEvents discards any queued pointer events for all menu tags.
// Must be called when the menu is not visible or was just shown
// to prevent stale events from firing.
func (m *ContextMenu) drainEvents(gtx layout.Context) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &m.bgTag, Kinds: pointer.Press}); !ok {
			break
		}
	}
	for _, tag := range m.itemTags {
		for {
			if _, ok := gtx.Event(pointer.Filter{
				Target: tag,
				Kinds:  pointer.Press | pointer.Enter | pointer.Leave,
			}); !ok {
				break
			}
		}
	}
}

// Layout renders the context menu overlay and returns any triggered action.
func (m *ContextMenu) Layout(gtx layout.Context, th *material.Theme) menuResult {
	if !m.visible {
		m.drainEvents(gtx)
		return menuResult{}
	}

	// On the frame Show() was called, drain stale bg events to prevent
	// immediate dismiss from events queued before Show().
	if m.showFrame {
		m.showFrame = false
		m.drainEvents(gtx)
	}

	totalH := menuItemH*len(m.items) + menuPadTop + menuPadBot

	// Compute display position: center first item at cursor, shift left slightly
	displayPos := clampPosition(
		image.Pt(m.pos.X-8, m.pos.Y-menuItemH/2-menuPadTop),
		menuW, totalH, gtx.Constraints.Max.X, gtx.Constraints.Max.Y,
	)

	// Background dismiss area: full window, with pass-through so row
	// handlers underneath still receive pointer events.
	passStack := pointer.PassOp{}.Push(gtx.Ops)
	bgArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &m.bgTag)
	bgArea.Pop()
	passStack.Pop()

	// Check for dismiss click (press outside menu bounds)
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &m.bgTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			menuRect := image.Rect(displayPos.X, displayPos.Y,
				displayPos.X+menuW, displayPos.Y+totalH)
			pt := image.Pt(int(e.Position.X), int(e.Position.Y))
			if !pt.In(menuRect) {
				m.Dismiss()
				return menuResult{}
			}
		}
	}

	pos := displayPos

	// Draw menu at position
	menuOffset := op.Offset(pos).Push(gtx.Ops)

	// Border
	borderRect := clip.Rect{Max: image.Pt(menuW, totalH)}.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, menuBorder, clip.Rect{Max: image.Pt(menuW, totalH)}.Op())
	borderRect.Pop()

	// Inner background
	innerRect := clip.Rect{Min: image.Pt(1, 1), Max: image.Pt(menuW-1, totalH-1)}.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, menuBG, clip.Rect{Max: image.Pt(menuW-2, totalH-2)}.Op())
	innerRect.Pop()

	// Draw items
	var result menuResult
	y := menuPadTop
	m.ensureTags()
	for i, item := range m.items {
		tag := m.itemTags[i]
		itemOff := op.Offset(image.Pt(1, y)).Push(gtx.Ops)
		itemW := menuW - 2

		// Event area for click + hover
		itemArea := clip.Rect{Max: image.Pt(itemW, menuItemH)}.Push(gtx.Ops)
		event.Op(gtx.Ops, tag)
		itemArea.Pop()

		// Process events (click + hover)
		for {
			ev, ok := gtx.Event(pointer.Filter{
				Target: tag,
				Kinds:  pointer.Press | pointer.Enter | pointer.Leave,
			})
			if !ok {
				break
			}
			if e, ok := ev.(pointer.Event); ok {
				switch e.Kind {
				case pointer.Enter:
					m.hoverIdx = i
				case pointer.Leave:
					if m.hoverIdx == i {
						m.hoverIdx = -1
					}
				case pointer.Press:
					result = menuResult{
						action: item.Action,
						pid:    m.targetPID,
						name:   m.targetName,
						ok:     true,
					}
					m.Dismiss()
				}
			}
		}

		// Hover background
		if m.hoverIdx == i {
			paint.FillShape(gtx.Ops, menuHoverBG,
				clip.Rect{Max: image.Pt(itemW, menuItemH)}.Op())
		}

		// Label
		labelOff := op.Offset(image.Pt(12, 7)).Push(gtx.Ops)

		labelColor := textPrimary
		if item.Action == ActionKill {
			labelColor = menuKillFG
		}
		if m.hoverIdx == i {
			labelColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		}

		l := material.Body2(th, item.Label)
		l.Color = labelColor
		l.TextSize = unit.Sp(13)
		l.Font.Weight = font.Normal
		l.Alignment = text.Start

		gtxLabel := gtx
		gtxLabel.Constraints = layout.Exact(image.Pt(itemW-24, menuItemH-14))
		l.Layout(gtxLabel)
		labelOff.Pop()

		itemOff.Pop()
		y += menuItemH
	}

	menuOffset.Pop()

	return result
}
