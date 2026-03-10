package proc

import (
	"fmt"
	"sort"

	"github.com/shirou/gopsutil/v4/process"
)

// Info holds resource usage information for a single process.
type Info struct {
	PID     int32
	Name    string // smart name derived from cmdline
	RawName string // original process name from OS
	CPU     float64
	RSS     uint64 // resident set size in bytes
	VMS     uint64 // virtual memory size in bytes
}

// Collect gathers process information for all running processes.
// Returns a slice sorted by CPU usage descending.
func Collect() ([]Info, error) {
	pids, err := process.Pids()
	if err != nil {
		return nil, fmt.Errorf("listing pids: %w", err)
	}

	infos := make([]Info, 0, len(pids))
	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err != nil {
			continue // process may have exited
		}

		info, err := gather(p)
		if err != nil {
			continue
		}

		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].CPU > infos[j].CPU
	})

	return infos, nil
}

// gather extracts process info from a gopsutil process handle.
func gather(p *process.Process) (Info, error) {
	name, err := p.Name()
	if err != nil {
		return Info{}, fmt.Errorf("getting name: %w", err)
	}

	cmdline, _ := p.CmdlineSlice()
	cwd, _ := p.Cwd()

	cpu, _ := p.CPUPercent()

	var rss, vms uint64
	mem, err := p.MemoryInfo()
	if err == nil && mem != nil {
		rss = mem.RSS
		vms = mem.VMS
	}

	return Info{
		PID:     p.Pid,
		Name:    SmartName(name, cmdline, cwd),
		RawName: name,
		CPU:     cpu,
		RSS:     rss,
		VMS:     vms,
	}, nil
}
