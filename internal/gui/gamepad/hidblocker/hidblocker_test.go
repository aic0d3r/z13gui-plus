package hidblocker

import (
	"os"
	"testing"
)

func TestHidrawMajor(t *testing.T) {
	major, err := hidrawMajor()
	if err != nil {
		t.Skipf("hidraw not available: %v", err)
	}
	if major == 0 {
		t.Fatal("hidraw major should be non-zero")
	}
	t.Logf("hidraw major: %d", major)
}

func TestNew(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root or CAP_BPF")
	}
	if !lsmEnabled() {
		t.Skip("BPF LSM not enabled")
	}

	b, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer b.Close()

	if err := b.Block(99999); err != nil {
		t.Fatalf("Block failed: %v", err)
	}
	b.UnblockAll()
}
