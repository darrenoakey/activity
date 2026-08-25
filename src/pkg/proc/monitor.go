package proc

import (
	"fmt"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"

	"github.com/shirou/gopsutil/v4/process"
)

// Monitor samples process CPU and memory at near-zero cost.
//
// Two ideas keep it flat:
//
//  1. Per-process identity (name, command line, working directory → smart
//     name) is investigated exactly once per PID lifetime and cached. The
//     expensive part of process monitoring on macOS is KERN_PROCARGS2 and
//     vnode-path lookups; none of it changes while a process runs.
//  2. CPU and memory are read with a single proc_pidinfo(PROC_PIDTASKINFO)
//     syscall per process — no per-call dlopen like the gopsutil hot path.
//
// PID reuse is detected by comparing the kernel's process start time; a
// mismatched start time re-runs the one-time investigation.
type Monitor struct {
	mu      sync.Mutex
	entries map[int32]*entry
	gen     uint64

	// investigations counts one-time identity lookups (for tests).
	investigations int
}

type entry struct {
	startSec  int64
	startUsec int32
	rawName   string
	name      string

	cpuTotal  float64   // cumulative CPU seconds at sampledAt
	sampledAt time.Time // zero until first observation
	gen       uint64
}

// NewMonitor creates a fresh sampler with an empty identity cache.
func NewMonitor() *Monitor {
	return &Monitor{entries: make(map[int32]*entry)}
}

// Sample gathers CPU and memory for every running process.
// Returns a slice sorted by CPU usage descending.
func (m *Monitor) Sample() ([]Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := initLibproc(); err != nil {
		return nil, fmt.Errorf("libproc: %w", err)
	}

	kprocs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}

	now := time.Now()
	m.gen++
	out := make([]Info, 0, len(kprocs))

	for i := range kprocs {
		kp := &kprocs[i]
		pid := kp.Proc.P_pid
		startSec := kp.Proc.P_starttime.Sec
		startUsec := int32(kp.Proc.P_starttime.Usec)

		e := m.entries[pid]
		if e == nil || e.startSec != startSec || e.startUsec != startUsec {
			// First sight of this PID (or the PID was reused): the only
			// time we ever pay for the expensive identification.
			e = m.investigate(pid, commString(kp.Proc.P_comm[:]))
			e.startSec, e.startUsec = startSec, startUsec
			m.entries[pid] = e
			m.investigations++
		}
		e.gen = m.gen

		var (
			cpu      float64
			rss, vms uint64
		)
		var ti taskInfo
		if procPidInfoFn(pid, procPidTaskInfo, 0, unsafe.Pointer(&ti), int32(unsafe.Sizeof(ti))) > 0 {
			total := (float64(ti.Total_user) + float64(ti.Total_system)) * tickToSec
			rss, vms = ti.Resident_size, ti.Virtual_size

			if !e.sampledAt.IsZero() {
				// Steady state: CPU over the window since the last sample.
				if dt := now.Sub(e.sampledAt).Seconds(); dt > 0 {
					cpu = 100 * (total - e.cpuTotal) / dt
					if cpu < 0 {
						cpu = 0 // counter reset on a PID we missed dying
					}
				}
			} else {
				// First observation: lifetime average, so a long-running
				// process shows a sane value before the next tick.
				age := now.Sub(time.Unix(startSec, int64(startUsec)*1000)).Seconds()
				if age > 0 {
					cpu = 100 * total / age
				}
			}
			e.cpuTotal, e.sampledAt = total, now
		}

		out = append(out, Info{
			PID:     pid,
			Name:    e.name,
			RawName: e.rawName,
			CPU:     cpu,
			RSS:     rss,
			VMS:     vms,
		})
	}

	// Prune identities of processes that no longer exist.
	for pid, e := range m.entries {
		if e.gen != m.gen {
			delete(m.entries, pid)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CPU > out[j].CPU })
	return out, nil
}

// investigate performs the one-time expensive identification of a process:
// OS name, full command line, and working directory, distilled into a smart
// name. comm is the kernel's short name, used as a fallback if the richer
// lookups fail.
func (m *Monitor) investigate(pid int32, comm string) *entry {
	e := &entry{rawName: comm, name: comm}

	p, err := process.NewProcess(pid)
	if err != nil {
		return e
	}

	name, err := p.Name()
	if err != nil || name == "" {
		name = comm
	}
	cmdline, _ := p.CmdlineSlice()
	cwd, _ := p.Cwd()

	e.rawName = name
	e.name = SmartName(name, cmdline, cwd)
	return e
}

// commString converts a NUL-padded kernel p_comm buffer to a Go string.
func commString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// ---- libproc bindings, resolved exactly once per process ----

const procPidTaskInfo = 4 // PROC_PIDTASKINFO flavor

// taskInfo mirrors struct proc_taskinfo from libproc.h.
type taskInfo struct {
	Virtual_size      uint64
	Resident_size     uint64
	Total_user        uint64
	Total_system      uint64
	Threads_user      uint64
	Threads_system    uint64
	Policy            int32
	Faults            int32
	Pageins           int32
	Cow_faults        int32
	Messages_sent     int32
	Messages_received int32
	Syscalls_mach     int32
	Syscalls_unix     int32
	Csw               int32
	Threadnum         int32
	Numrunning        int32
	Priority          int32
}

type machTimebaseInfo struct {
	Numer uint32
	Denom uint32
}

var (
	libprocOnce   sync.Once
	libprocErr    error
	procPidInfoFn func(pid int32, flavor int32, arg uint64, buf unsafe.Pointer, bufsize int32) int32
	tickToSec     float64 // mach ticks → seconds
)

func initLibproc() error {
	libprocOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			libprocErr = fmt.Errorf("opening libSystem: %w", err)
			return
		}
		purego.RegisterLibFunc(&procPidInfoFn, lib, "proc_pidinfo")
		var machTimebaseFn func(*machTimebaseInfo) uint32
		purego.RegisterLibFunc(&machTimebaseFn, lib, "mach_timebase_info")
		tb := machTimebaseInfo{}
		machTimebaseFn(&tb)
		if tb.Denom == 0 {
			libprocErr = fmt.Errorf("mach_timebase_info returned zero denominator")
			return
		}
		tickToSec = float64(tb.Numer) / float64(tb.Denom) / 1e9
	})
	return libprocErr
}
