package proc

import (
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func TestMonitorSample(t *testing.T) {
	m := NewMonitor()

	infos, err := m.Sample()
	if err != nil {
		t.Fatalf("Sample() error: %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("Sample() returned no processes")
	}

	myPID := int32(os.Getpid())
	found := false
	for _, info := range infos {
		if info.PID == myPID {
			found = true
			if info.Name == "" {
				t.Error("own process has empty Name")
			}
		}
	}
	if !found {
		t.Errorf("own process (pid %d) not found in Sample() results", myPID)
	}

	// Verify sorting: CPU should be descending
	for i := 1; i < len(infos); i++ {
		if infos[i-1].CPU < infos[i].CPU {
			t.Fatalf("Sample() not sorted by CPU desc: [%d]=%.2f < [%d]=%.2f",
				i-1, infos[i-1].CPU, i, infos[i].CPU)
		}
	}
}

func TestMonitorIdentityCached(t *testing.T) {
	m := NewMonitor()

	if _, err := m.Sample(); err != nil {
		t.Fatalf("first Sample() error: %v", err)
	}
	myPID := int32(os.Getpid())
	first, ok := m.entries[myPID]
	if !ok {
		t.Fatalf("own pid %d missing from identity cache", myPID)
	}
	if first.name == "" {
		t.Fatal("cached identity has empty name")
	}

	if _, err := m.Sample(); err != nil {
		t.Fatalf("second Sample() error: %v", err)
	}
	second, ok := m.entries[myPID]
	if !ok {
		t.Fatalf("own pid %d pruned while still running", myPID)
	}
	if first != second {
		t.Error("identity re-investigated for a still-running process; must be cached")
	}

	// A mismatched start time (PID reuse) must trigger re-investigation.
	savedSec, savedUsec := second.startSec, second.startUsec
	second.startSec = -1
	if _, err := m.Sample(); err != nil {
		t.Fatalf("third Sample() error: %v", err)
	}
	replaced, ok := m.entries[myPID]
	if !ok || replaced == second {
		t.Error("stale identity not re-investigated after start-time mismatch")
	}
	if replaced.startSec != savedSec || replaced.startUsec != savedUsec {
		t.Error("re-investigated entry did not adopt the kernel's start time")
	}
}

func TestMonitorCPUDelta(t *testing.T) {
	m := NewMonitor()
	if _, err := m.Sample(); err != nil {
		t.Fatalf("baseline Sample() error: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := 1.1
			for {
				select {
				case <-stop:
					return
				default:
					for range 10000 {
						x = x*1.0000001 + 1
					}
				}
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)
	infos, err := m.Sample()
	close(stop)
	wg.Wait()
	if err != nil {
		t.Fatalf("second Sample() error: %v", err)
	}

	myPID := int32(os.Getpid())
	for _, info := range infos {
		if info.PID == myPID {
			if info.CPU < 50 {
				t.Errorf("own pid CPU = %.1f%% during busy loop, want >= 50%%", info.CPU)
			}
			return
		}
	}
	t.Fatalf("own pid %d not found", myPID)
}

func TestMonitorPrunesDead(t *testing.T) {
	m := NewMonitor()

	cmd := exec.Command("sleep", "2")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting sleep: %v", err)
	}
	pid := int32(cmd.Process.Pid)

	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := m.Sample(); err != nil {
			t.Fatalf("Sample() error: %v", err)
		}
		if _, ok := m.entries[pid]; ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("sleep pid %d never appeared in cache", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting for sleep: %v", err)
	}

	if _, err := m.Sample(); err != nil {
		t.Fatalf("Sample() error: %v", err)
	}
	if e, ok := m.entries[pid]; ok {
		t.Errorf("dead pid %d still cached (name %q)", pid, e.name)
	}
}
