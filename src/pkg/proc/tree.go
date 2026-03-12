package proc

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/process"
)

// TreeNode represents a single process in the process tree.
type TreeNode struct {
	PID      int32
	Name     string
	PPID     int32
	Children []*TreeNode
	IsTarget bool
}

// BuildTree constructs the process tree rooted at the ancestor of targetPID.
// The target node is marked with IsTarget=true.
// Returns the root of the tree containing the target process.
func BuildTree(targetPID int32) (*TreeNode, error) {
	pids, err := process.Pids()
	if err != nil {
		return nil, fmt.Errorf("listing pids: %w", err)
	}

	nodes := make(map[int32]*TreeNode, len(pids))
	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err != nil {
			continue
		}

		name, err := p.Name()
		if err != nil {
			name = ""
		}

		cmdline, _ := p.CmdlineSlice()
		cwd, _ := p.Cwd()

		ppid, err := p.Ppid()
		if err != nil {
			ppid = 0
		}

		nodes[pid] = &TreeNode{
			PID:  pid,
			Name: SmartName(name, cmdline, cwd),
			PPID: ppid,
		}
	}

	target, ok := nodes[targetPID]
	if !ok {
		return nil, fmt.Errorf("target pid %d not found", targetPID)
	}
	target.IsTarget = true

	// link children to parents
	for _, node := range nodes {
		if node.PID == node.PPID {
			// init/launchd reports itself as parent — skip to avoid cycles
			continue
		}
		if parent, exists := nodes[node.PPID]; exists {
			parent.Children = append(parent.Children, node)
		}
	}

	// Build the ancestor chain: walk up from target to root.
	// Each ancestor only keeps the single child on the path to the target.
	// The target itself keeps ALL its children (full subtree below).
	var chain []*TreeNode
	cur := target
	for {
		chain = append(chain, cur)
		parent, exists := nodes[cur.PPID]
		if !exists || cur.PPID == 0 || cur.PPID == cur.PID {
			break
		}
		cur = parent
	}

	// Prune: for each ancestor (not the target), only keep the child
	// that leads toward the target.
	for i := len(chain) - 1; i > 0; i-- {
		ancestor := chain[i]
		childOnPath := chain[i-1]
		ancestor.Children = []*TreeNode{childOnPath}
	}
	// Target (chain[0]) keeps all its children — no pruning.

	return chain[len(chain)-1], nil
}
