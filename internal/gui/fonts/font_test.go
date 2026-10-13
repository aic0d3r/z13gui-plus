package fonts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPrivateDir(t *testing.T) {
	tmp := t.TempDir()
	private := filepath.Join(tmp, "private")
	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkPrivateDir(private); err != nil {
		t.Fatalf("private dir rejected: %v", err)
	}

	open := filepath.Join(tmp, "open")
	if err := os.Mkdir(open, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o777); err != nil {
		t.Fatal(err)
	}
	if checkPrivateDir(open) == nil {
		t.Error("world-writable dir accepted")
	}

	link := filepath.Join(tmp, "link")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	if checkPrivateDir(link) == nil {
		t.Error("symlink to dir accepted")
	}
}
