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
)

// ContextMenu is a floating right-click context menu.
type ContextMenu struct {
	visible    bool
	pos        image.Point // absolute window position
	targetPID  int32
	targetName string
	items      []menuItem
	itemTags   [3]bool // used as pointer event tags (address = tag)
	bgTag      bool    // background dismiss tag
}

type menuItem struct {
	label  string
	action MenuAction
}

var defaultMenuItems = []menuItem{
	{"Hide", ActionHide},
	{"Tree", ActionTree},
	{"Info", ActionInfo},
}

// Show opens the context menu at the given window position for the given process.
func (m *ContextMenu) Show(pos image.Point, pid int32, name string) {
	m.visible = true
	m.pos = pos
	m.targetPID = pid
	m.targetName = name
	m.items = defaultMenuItems
}

// Dismiss closes the context menu.
func (m *ContextMenu) Dismiss() {
	m.visible = false
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
	menuBG     = color.NRGBA{R: 0x28, G: 0x28, B: 0x28, A: 0xf0}
	menuBorder = color.NRGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff}
)

// drainEvents discards any queued pointer events for all menu tags.
// Must be called when the menu is not visible to prevent stale events
// from firing when the menu is next shown.
func (m *ContextMenu) drainEvents(gtx layout.Context) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &m.bgTag, Kinds: pointer.Press}); !ok {
			break
		}
	}
	for i := range m.itemTags {
		for {
			if _, ok := gtx.Event(pointer.Filter{Target: &m.itemTags[i], Kinds: pointer.Press}); !ok {
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

	// Background dismiss area: full window, to detect clicks outside menu
	bgArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &m.bgTag)
	bgArea.Pop()

	// Check for dismiss click (press on background outside menu bounds)
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &m.bgTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			menuRect := image.Rect(m.pos.X, m.pos.Y,
				m.pos.X+menuW, m.pos.Y+menuItemH*len(m.items)+menuPadTop+menuPadBot)
			pt := image.Pt(int(e.Position.X), int(e.Position.Y))
			if !pt.In(menuRect) {
				m.Dismiss()
				return menuResult{}
			}
		}
	}

	// Clamp menu position to stay within window
	pos := m.pos
	if maxX := gtx.Constraints.Max.X - menuW; pos.X > maxX {
		pos.X = maxX
	}
	totalH := menuItemH*len(m.items) + menuPadTop + menuPadBot
	if maxY := gtx.Constraints.Max.Y - totalH; pos.Y > maxY {
		pos.Y = maxY
	}
	if pos.X < 0 {
		pos.X = 0
	}
	if pos.Y < 0 {
		pos.Y = 0
	}

	// Draw menu at position
	menuOffset := op.Offset(pos).Push(gtx.Ops)

	// Menu background with border
	borderRect := clip.Rect{Max: image.Pt(menuW, totalH)}.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, menuBorder, clip.Rect{Max: image.Pt(menuW, totalH)}.Op())
	borderRect.Pop()

	innerRect := clip.Rect{Min: image.Pt(1, 1), Max: image.Pt(menuW-1, totalH-1)}.Push(gtx.Ops)
	paint.FillShape(gtx.Ops, menuBG, clip.Rect{Max: image.Pt(menuW-2, totalH-2)}.Op())
	innerRect.Pop()

	// Draw menu items
	var result menuResult
	y := menuPadTop
	for i, item := range m.items {
		itemOff := op.Offset(image.Pt(1, y)).Push(gtx.Ops)
		itemW := menuW - 2

		// Item click area
		itemArea := clip.Rect{Max: image.Pt(itemW, menuItemH)}.Push(gtx.Ops)
		event.Op(gtx.Ops, &m.itemTags[i])
		itemArea.Pop()

		// Check for click on this item
		for {
			ev, ok := gtx.Event(pointer.Filter{
				Target: &m.itemTags[i],
				Kinds:  pointer.Press,
			})
			if !ok {
				break
			}
			if _, ok := ev.(pointer.Event); ok {
				result = menuResult{
					action: item.action,
					pid:    m.targetPID,
					name:   m.targetName,
					ok:     true,
				}
				m.Dismiss()
			}
		}

		// Label
		labelOff := op.Offset(image.Pt(12, 7)).Push(gtx.Ops)
		l := material.Body2(th, item.label)
		l.Color = textPrimary
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
