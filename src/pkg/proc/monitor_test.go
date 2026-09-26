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

func TestMonitorAccumulatesTotals(t *testing.T) {
	m := NewMonitor()
	myPID := int32(os.Getpid())

	first := sampleOwn(t, m, myPID)
	if first.Samples != 1 {
		t.Fatalf("Samples = %d, want 1", first.Samples)
	}
	if first.RSSSum != first.RSS {
		t.Fatalf("RSSSum %d != first RSS %d", first.RSSSum, first.RSS)
	}
	if first.AverageRSS() != first.RSS {
		t.Fatalf("AverageRSS %d != RSS %d", first.AverageRSS(), first.RSS)
	}
	if m.SampleCount() != 1 {
		t.Fatalf("SampleCount = %d, want 1", m.SampleCount())
	}

	second := sampleOwn(t, m, myPID)
	if second.Samples != 2 {
		t.Fatalf("Samples = %d after second sample, want 2", second.Samples)
	}
	if second.RSSSum != first.RSS+second.RSS {
		t.Fatalf("RSSSum %d != %d + %d", second.RSSSum, first.RSS, second.RSS)
	}
	wantAvg := (first.RSS + second.RSS) / 2
	if second.AverageRSS() != wantAvg {
		t.Fatalf("AverageRSS = %d, want %d", second.AverageRSS(), wantAvg)
	}
	if second.CPUSum < first.CPU {
		t.Fatalf("CPUSum %f shrank from first CPU %f", second.CPUSum, first.CPU)
	}
	if m.SampleCount() != 2 {
		t.Fatalf("SampleCount = %d, want 2", m.SampleCount())
	}
}

func TestMonitorDropsTotalsWithDeadProcess(t *testing.T) {
	m := NewMonitor()
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting sleep: %v", err)
	}
	pid := int32(cmd.Process.Pid)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	seen := sampleOwn(t, m, pid)
	if seen.Samples < 1 || seen.RSSSum == 0 && seen.RSS == 0 {
		// sleep has a resident set; a zero reading means we never observed it.
		if seen.Samples < 1 {
			t.Fatalf("sleep pid %d was not accumulated", pid)
		}
	}
	if seen.Samples < 1 {
		t.Fatalf("sleep pid %d Samples = 0", pid)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("killing sleep: %v", err)
	}
	_, _ = cmd.Process.Wait()

	deadline := time.Now().Add(3 * time.Second)
	for {
		infos, err := m.Sample()
		if err != nil {
			t.Fatalf("Sample() error: %v", err)
		}
		if !containsPID(infos, pid) {
			if _, ok := m.entries[pid]; ok {
				t.Fatalf("dead pid %d still holds totals", pid)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid %d still listed after kill", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestAverageRSSZeroWhenUnsampled(t *testing.T) {
	var info Info
	if info.AverageRSS() != 0 || info.AverageCPU() != 0 {
		t.Fatal("unsampled averages must be zero, not a divide-by-zero")
	}
}

func sampleOwn(t *testing.T, m *Monitor, pid int32) Info {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		infos, err := m.Sample()
		if err != nil {
			t.Fatalf("Sample() error: %v", err)
		}
		for _, info := range infos {
			if info.PID == pid && info.Samples > 0 {
				return info
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid %d never accumulated a sample", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func containsPID(infos []Info, pid int32) bool {
	for _, info := range infos {
		if info.PID == pid {
			return true
		}
	}
	return false
}
