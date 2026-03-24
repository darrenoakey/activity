package ui

import (
	"image"
	"testing"
)

// Tests for processMenu — the app-specific wrapper over menu.ContextMenu.
// Core menu behavior (clamping, hover, tags, dimensions) is tested in the
// daz-golang-gio/menu package.

func TestProcessMenu_ShowSetsTarget(t *testing.T) {
	var m processMenu
	m.Show(image.Pt(100, 200), 1234, "test-proc")
	if m.targetPID != 1234 {
		t.Errorf("targetPID = %d, want 1234", m.targetPID)
	}
	if m.targetName != "test-proc" {
		t.Errorf("targetName = %q, want %q", m.targetName, "test-proc")
	}
	if !m.Visible() {
		t.Error("menu should be visible after Show")
	}
}

func TestProcessMenu_ShowUpdatesTarget(t *testing.T) {
	var m processMenu
	m.Show(image.Pt(100, 100), 1000, "first")
	m.Show(image.Pt(200, 200), 2000, "second")
	if m.targetPID != 2000 {
		t.Errorf("targetPID = %d, want 2000", m.targetPID)
	}
	if m.targetName != "second" {
		t.Errorf("targetName = %q, want %q", m.targetName, "second")
	}
}

func TestProcessMenu_DismissPreservesTarget(t *testing.T) {
	var m processMenu
	m.Show(image.Pt(100, 200), 1234, "proc")
	m.Dismiss()
	if m.Visible() {
		t.Error("menu should not be visible after Dismiss")
	}
	// Target info preserved for potential reuse
	if m.targetPID != 1234 {
		t.Errorf("targetPID = %d, want 1234 (preserved after dismiss)", m.targetPID)
	}
}

func TestProcessMenu_ShowDismissShowCycle(t *testing.T) {
	var m processMenu

	m.Show(image.Pt(100, 100), 1000, "proc1")
	if !m.Visible() {
		t.Fatal("should be visible after first Show")
	}

	m.Dismiss()

	m.Show(image.Pt(200, 200), 2000, "proc2")
	if !m.Visible() {
		t.Error("should be visible after second Show")
	}
	if m.targetPID != 2000 {
		t.Errorf("targetPID = %d, want 2000 after second Show", m.targetPID)
	}
}

func TestDefaultMenuItems_HasExpectedLabels(t *testing.T) {
	expected := []string{"Hide", "Tree", "Info", "Kill"}
	if len(defaultMenuItems) != len(expected) {
		t.Fatalf("defaultMenuItems count = %d, want %d", len(defaultMenuItems), len(expected))
	}
	for i, label := range expected {
		if defaultMenuItems[i].Label != label {
			t.Errorf("defaultMenuItems[%d].Label = %q, want %q", i, defaultMenuItems[i].Label, label)
		}
	}
}

func TestDefaultMenuItems_KillHasRedColor(t *testing.T) {
	killItem := defaultMenuItems[actionKill]
	if killItem.Color.A == 0 {
		t.Error("Kill item should have a custom color (red)")
	}
	if killItem.Color != menuKillFG {
		t.Errorf("Kill color = %v, want %v", killItem.Color, menuKillFG)
	}
}

func TestActionIndices(t *testing.T) {
	// Verify action constants match defaultMenuItems positions
	if defaultMenuItems[actionHide].Label != "Hide" {
		t.Errorf("actionHide (%d) doesn't point to Hide", actionHide)
	}
	if defaultMenuItems[actionTree].Label != "Tree" {
		t.Errorf("actionTree (%d) doesn't point to Tree", actionTree)
	}
	if defaultMenuItems[actionInfo].Label != "Info" {
		t.Errorf("actionInfo (%d) doesn't point to Info", actionInfo)
	}
	if defaultMenuItems[actionKill].Label != "Kill" {
		t.Errorf("actionKill (%d) doesn't point to Kill", actionKill)
	}
}
