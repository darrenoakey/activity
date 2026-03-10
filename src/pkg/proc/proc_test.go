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
