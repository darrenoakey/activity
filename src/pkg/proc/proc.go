package proc

import (
	"fmt"
	"sort"

	"github.com/shirou/gopsutil/v4/process"
)

// ExtendedInfo holds detailed information about a single process.
type ExtendedInfo struct {
	Info
	Cmdline    string
	CmdSlice   []string
	Cwd        string
	Status     string
	Username   string
	NumThreads int32
	CreateTime int64 // Unix millis
	Ppid       int32
	Environ    []string
}

// FetchExtended retrieves detailed process information for the given PID.
// Individual field errors are ignored — fields are populated on a best-effort basis.
func FetchExtended(pid int32) (ExtendedInfo, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return ExtendedInfo{}, fmt.Errorf("opening process %d: %w", pid, err)
	}

	name, err := p.Name()
	if err != nil {
		return ExtendedInfo{}, fmt.Errorf("getting name for pid %d: %w", pid, err)
	}

	cmdSlice, _ := p.CmdlineSlice()
	cwd, _ := p.Cwd()

	cpu, _ := p.CPUPercent()

	var rss, vms uint64
	mem, err := p.MemoryInfo()
	if err == nil && mem != nil {
		rss = mem.RSS
		vms = mem.VMS
	}

	cmdline, _ := p.Cmdline()

	statuses, _ := p.Status()
	var status string
	if len(statuses) > 0 {
		status = statuses[0]
	}

	username, _ := p.Username()

	var numThreads int32
	if n, err := p.NumThreads(); err == nil {
		numThreads = n
	}

	var createTime int64
	if ct, err := p.CreateTime(); err == nil {
		createTime = ct
	}

	var ppid int32
	if pp, err := p.Ppid(); err == nil {
		ppid = pp
	}

	environ, _ := p.Environ()

	return ExtendedInfo{
		Info: Info{
			PID:     p.Pid,
			Name:    SmartName(name, cmdSlice, cwd),
			RawName: name,
			CPU:     cpu,
			RSS:     rss,
			VMS:     vms,
		},
		Cmdline:    cmdline,
		CmdSlice:   cmdSlice,
		Cwd:        cwd,
		Status:     status,
		Username:   username,
		NumThreads: numThreads,
		CreateTime: createTime,
		Ppid:       ppid,
		Environ:    environ,
	}, nil
}

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
