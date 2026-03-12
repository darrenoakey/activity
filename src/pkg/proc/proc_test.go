package proc

import (
	"os"
	"testing"
)

func TestCollect(t *testing.T) {
	infos, err := Collect()
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(infos) == 0 {
		t.Fatal("Collect() returned no processes")
	}

	// Verify our own process appears in the list
	myPID := int32(os.Getpid())
	found := false
	for _, info := range infos {
		if info.PID == myPID {
			found = true
			if info.Name == "" {
				t.Error("own process has empty name")
			}
			break
		}
	}
	if !found {
		t.Errorf("own process (pid %d) not found in Collect() results", myPID)
	}

	// Verify sorting: CPU should be descending
	for i := 1; i < len(infos); i++ {
		if infos[i].CPU > infos[i-1].CPU {
			t.Errorf("processes not sorted by CPU descending at index %d: %f > %f",
				i, infos[i].CPU, infos[i-1].CPU)
			break
		}
	}
}

func TestFetchExtended(t *testing.T) {
	myPID := int32(os.Getpid())

	ext, err := FetchExtended(myPID)
	if err != nil {
		t.Fatalf("FetchExtended(%d) error: %v", myPID, err)
	}

	if ext.PID != myPID {
		t.Errorf("FetchExtended PID = %d, want %d", ext.PID, myPID)
	}

	if ext.Name == "" {
		t.Error("FetchExtended returned empty Name")
	}

	// CmdSlice should include the test binary path
	if len(ext.CmdSlice) == 0 {
		t.Error("FetchExtended returned empty CmdSlice")
	}
}

func TestBuildTree(t *testing.T) {
	myPID := int32(os.Getpid())

	root, err := BuildTree(myPID)
	if err != nil {
		t.Fatalf("BuildTree(%d) error: %v", myPID, err)
	}

	if root == nil {
		t.Fatal("BuildTree returned nil root")
	}

	// Find the target node in the tree
	target := findNode(root, myPID)
	if target == nil {
		t.Fatalf("target pid %d not found in tree rooted at pid %d", myPID, root.PID)
	}

	if !target.IsTarget {
		t.Error("target node IsTarget should be true")
	}

	// Root should have a non-empty name
	if root.Name == "" {
		t.Error("root node has empty Name")
	}

	// Root must be an ancestor of target (or be the target itself)
	if root.PID != myPID {
		// tree should trace back from root to target via children
		if target == nil {
			t.Errorf("target pid %d not reachable from root pid %d", myPID, root.PID)
		}
	}
}

// findNode does a depth-first search for a node with the given PID.
func findNode(node *TreeNode, pid int32) *TreeNode {
	if node == nil {
		return nil
	}
	if node.PID == pid {
		return node
	}
	for _, child := range node.Children {
		if found := findNode(child, pid); found != nil {
			return found
		}
	}
	return nil
}
