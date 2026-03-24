package ui

import (
	"image"
	"image/color"

	"github.com/darrenoakey/daz-golang-gio/menu"

	"gioui.org/layout"
	"gioui.org/widget/material"
)

// Menu action indices (positions in menuItems slice).
const (
	actionHide = iota
	actionTree
	actionInfo
	actionKill
)

var menuKillFG = color.NRGBA{R: 0xff, G: 0x5c, B: 0x5c, A: 0xff}

// defaultMenuItems is the standard set of context menu actions.
var defaultMenuItems = []menu.Item{
	{Label: "Hide"},
	{Label: "Tree"},
	{Label: "Info"},
	{Label: "Kill", Color: menuKillFG},
}

// processMenu wraps menu.ContextMenu with process-specific target tracking.
type processMenu struct {
	menu.ContextMenu
	targetPID  int32
	targetName string
}

// menuResult is returned when an action is triggered.
type menuResult struct {
	action int
	pid    int32
	name   string
	ok     bool
}

// Show opens the context menu for the given process.
func (m *processMenu) Show(pos image.Point, pid int32, name string) {
	m.targetPID = pid
	m.targetName = name
	m.ContextMenu.Show(pos, defaultMenuItems)
}

// ShowItems opens the context menu with custom items for the given process.
func (m *processMenu) ShowItems(pos image.Point, pid int32, name string, items []menu.Item) {
	m.targetPID = pid
	m.targetName = name
	m.ContextMenu.Show(pos, items)
}

// Layout renders the menu and returns any triggered action with process context.
func (m *processMenu) Layout(gtx layout.Context, th *material.Theme) menuResult {
	result := m.ContextMenu.Layout(gtx, th)
	if !result.OK {
		return menuResult{}
	}
	return menuResult{
		action: result.Index,
		pid:    m.targetPID,
		name:   m.targetName,
		ok:     true,
	}
}
