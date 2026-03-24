package ui

import (
	"image"
	"testing"
)

// --- Lifecycle: Show / Dismiss ---

func TestContextMenu_ShowMakesVisible(t *testing.T) {
	var m ContextMenu
	if m.Visible() {
		t.Fatal("new menu should not be visible")
	}
	m.Show(image.Pt(100, 200), 1234, "test-proc")
	if !m.Visible() {
		t.Error("menu should be visible after Show")
	}
}

func TestContextMenu_ShowSetsTarget(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(100, 200), 1234, "test-proc")
	if m.targetPID != 1234 {
		t.Errorf("targetPID = %d, want 1234", m.targetPID)
	}
	if m.targetName != "test-proc" {
		t.Errorf("targetName = %q, want %q", m.targetName, "test-proc")
	}
	if m.pos != (image.Point{X: 100, Y: 200}) {
		t.Errorf("pos = %v, want (100,200)", m.pos)
	}
}

func TestContextMenu_ShowPopulatesDefaultItems(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(0, 0), 1, "p")
	if len(m.items) != len(DefaultMenuItems) {
		t.Fatalf("items count = %d, want %d", len(m.items), len(DefaultMenuItems))
	}
	for i, item := range m.items {
		if item.Label != DefaultMenuItems[i].Label || item.Action != DefaultMenuItems[i].Action {
			t.Errorf("item[%d] = %+v, want %+v", i, item, DefaultMenuItems[i])
		}
	}
}

func TestContextMenu_ShowResetsHover(t *testing.T) {
	var m ContextMenu
	m.hoverIdx = 2
	m.Show(image.Pt(0, 0), 1, "p")
	if m.hoverIdx != -1 {
		t.Errorf("hoverIdx = %d, want -1 after Show", m.hoverIdx)
	}
}

func TestContextMenu_ShowSetsShowFrame(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(0, 0), 1, "p")
	if !m.showFrame {
		t.Error("showFrame should be true after Show")
	}
}

func TestContextMenu_DismissClearsVisible(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(100, 200), 1234, "proc")
	m.Dismiss()
	if m.Visible() {
		t.Error("menu should not be visible after Dismiss")
	}
}

func TestContextMenu_DismissResetsHover(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(0, 0), 1, "p")
	m.hoverIdx = 2
	m.Dismiss()
	if m.hoverIdx != -1 {
		t.Errorf("hoverIdx = %d, want -1 after Dismiss", m.hoverIdx)
	}
}

func TestContextMenu_ShowUpdatesExistingTarget(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(100, 100), 1000, "first")
	m.Show(image.Pt(200, 200), 2000, "second")
	if m.targetPID != 2000 {
		t.Errorf("targetPID = %d, want 2000", m.targetPID)
	}
	if m.targetName != "second" {
		t.Errorf("targetName = %q, want %q", m.targetName, "second")
	}
	if m.pos != (image.Point{X: 200, Y: 200}) {
		t.Errorf("pos = %v, want (200,200)", m.pos)
	}
}

// --- ShowFrame bug fix: Show-Dismiss-Show cycle ---

func TestContextMenu_ShowDismissShowCycle(t *testing.T) {
	var m ContextMenu

	// First show
	m.Show(image.Pt(100, 100), 1000, "proc1")
	if !m.Visible() {
		t.Fatal("should be visible after first Show")
	}
	if !m.showFrame {
		t.Error("showFrame should be true after first Show")
	}

	// Simulate frame processing
	m.showFrame = false

	// Dismiss
	m.Dismiss()
	if m.Visible() {
		t.Fatal("should not be visible after Dismiss")
	}

	// Second show (the bug: this must work immediately)
	m.Show(image.Pt(200, 200), 2000, "proc2")
	if !m.Visible() {
		t.Error("should be visible after second Show")
	}
	if !m.showFrame {
		t.Error("showFrame must be true after second Show (prevents stale bg dismiss)")
	}
}

// --- Position clamping ---

func TestClampPosition_WithinBounds(t *testing.T) {
	pos := clampPosition(image.Pt(50, 50), 120, 100, 800, 600)
	if pos != (image.Point{X: 50, Y: 50}) {
		t.Errorf("pos = %v, want (50,50)", pos)
	}
}

func TestClampPosition_RightOverflow(t *testing.T) {
	pos := clampPosition(image.Pt(750, 50), 120, 100, 800, 600)
	if pos.X != 680 {
		t.Errorf("pos.X = %d, want 680", pos.X)
	}
	if pos.Y != 50 {
		t.Errorf("pos.Y = %d, want 50 (unchanged)", pos.Y)
	}
}

func TestClampPosition_BottomOverflow(t *testing.T) {
	pos := clampPosition(image.Pt(50, 550), 120, 100, 800, 600)
	if pos.X != 50 {
		t.Errorf("pos.X = %d, want 50 (unchanged)", pos.X)
	}
	if pos.Y != 500 {
		t.Errorf("pos.Y = %d, want 500", pos.Y)
	}
}

func TestClampPosition_NegativeCoords(t *testing.T) {
	pos := clampPosition(image.Pt(-10, -20), 120, 100, 800, 600)
	if pos.X != 0 || pos.Y != 0 {
		t.Errorf("pos = %v, want (0,0)", pos)
	}
}

func TestClampPosition_CornerOverflow(t *testing.T) {
	pos := clampPosition(image.Pt(750, 550), 120, 100, 800, 600)
	if pos.X != 680 || pos.Y != 500 {
		t.Errorf("pos = %v, want (680,500)", pos)
	}
}

func TestClampPosition_ExactFit(t *testing.T) {
	// Menu exactly fills the window
	pos := clampPosition(image.Pt(0, 0), 800, 600, 800, 600)
	if pos.X != 0 || pos.Y != 0 {
		t.Errorf("pos = %v, want (0,0)", pos)
	}
}

func TestClampPosition_LargerThanWindow(t *testing.T) {
	// Menu wider/taller than window clamps to 0,0
	pos := clampPosition(image.Pt(100, 100), 900, 700, 800, 600)
	if pos.X != 0 || pos.Y != 0 {
		t.Errorf("pos = %v, want (0,0) when menu exceeds window", pos)
	}
}

// --- Items configuration ---

func TestDefaultMenuItems_HasAllActions(t *testing.T) {
	expected := map[MenuAction]string{
		ActionHide: "Hide",
		ActionTree: "Tree",
		ActionInfo: "Info",
		ActionKill: "Kill",
	}
	for action, label := range expected {
		found := false
		for _, item := range DefaultMenuItems {
			if item.Action == action && item.Label == label {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultMenuItems missing action %d with label %q", action, label)
		}
	}
}

func TestDefaultMenuItems_UniqueActions(t *testing.T) {
	seen := make(map[MenuAction]bool)
	for _, item := range DefaultMenuItems {
		if seen[item.Action] {
			t.Errorf("duplicate action value: %d (%s)", item.Action, item.Label)
		}
		seen[item.Action] = true
	}
}

func TestDefaultMenuItems_KillIsDestructive(t *testing.T) {
	// Kill should be last (conventional placement for destructive actions)
	last := DefaultMenuItems[len(DefaultMenuItems)-1]
	if last.Action != ActionKill {
		t.Errorf("last item action = %d, want ActionKill (%d)", last.Action, ActionKill)
	}
}

// --- Tag management ---

func TestContextMenu_EnsureTagsAllocates(t *testing.T) {
	var m ContextMenu
	m.items = DefaultMenuItems
	m.ensureTags()
	if len(m.itemTags) < len(m.items) {
		t.Errorf("itemTags len = %d, want >= %d", len(m.itemTags), len(m.items))
	}
}

func TestContextMenu_EnsureTagsStable(t *testing.T) {
	var m ContextMenu
	m.items = DefaultMenuItems
	m.ensureTags()
	first := make([]*bool, len(m.itemTags))
	copy(first, m.itemTags)

	// Second call should not reallocate existing tags
	m.ensureTags()
	for i, tag := range m.itemTags[:len(first)] {
		if tag != first[i] {
			t.Errorf("tag[%d] pointer changed after second ensureTags", i)
		}
	}
}

func TestContextMenu_EnsureTagsUniquePointers(t *testing.T) {
	var m ContextMenu
	m.items = DefaultMenuItems
	m.ensureTags()
	seen := make(map[*bool]bool)
	for _, tag := range m.itemTags {
		if seen[tag] {
			t.Error("duplicate tag pointer found")
		}
		seen[tag] = true
	}
}

// --- Menu dimensions ---

func TestMenuDimensions(t *testing.T) {
	items := DefaultMenuItems
	expectedH := menuItemH*len(items) + menuPadTop + menuPadBot
	if expectedH != 136 { // 32*4 + 4 + 4
		t.Errorf("total menu height = %d, want 136", expectedH)
	}
	if menuW != 120 {
		t.Errorf("menu width = %d, want 120", menuW)
	}
}

// --- Hover state ---

func TestContextMenu_HoverInitialState(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(0, 0), 1, "p")
	if m.hoverIdx != -1 {
		t.Errorf("hoverIdx = %d, want -1 (no hover initially)", m.hoverIdx)
	}
}

func TestContextMenu_HoverClearedOnReshow(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(0, 0), 1, "p")
	m.hoverIdx = 2 // simulate cursor hovering item 2
	m.Show(image.Pt(50, 50), 2, "q")
	if m.hoverIdx != -1 {
		t.Errorf("hoverIdx = %d, want -1 after re-Show", m.hoverIdx)
	}
}

func TestContextMenu_HoverClearedOnDismiss(t *testing.T) {
	var m ContextMenu
	m.Show(image.Pt(0, 0), 1, "p")
	m.hoverIdx = 1
	m.Dismiss()
	if m.hoverIdx != -1 {
		t.Errorf("hoverIdx = %d, want -1 after Dismiss", m.hoverIdx)
	}
}
