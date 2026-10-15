package gamepad

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/aic0d3r/z13gui-plus/internal/gui/gamepad/hidblocker"
)

// pidFilePath returns the path to the frozen-PID state file.
// Used for crash recovery: if Z13GUI+ is killed, the systemd ExecStopPost
// or signal handler reads this file to thaw the frozen Steam process.
func pidFilePath() string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(runtime, "z13gui-plus", "frozen-pid")
}

// FindSteamPID locates the main Steam process by scanning /proc.
// Returns 0 if not found (graceful degradation — no freeze attempted).
func FindSteamPID() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(comm)) == "steam" {
			slog.Debug("steam: found process", "pid", pid)
			return pid
		}
	}
	return 0
}

// FreezeProc sends SIGSTOP to suspend a process and writes the PID to a
// state file for crash recovery.
func FreezeProc(pid int) error {
	path := pidFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		return err
	}
	_ = os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600)
	return nil
}

// ThawProc sends SIGCONT to resume a process and removes the state file.
func ThawProc(pid int) error {
	_ = os.Remove(pidFilePath())
	return syscall.Kill(pid, syscall.SIGCONT)
}

// ThawFrozen reads the PID state file and sends SIGCONT if a frozen PID is
// recorded. Used by signal handlers and cleanup paths for crash recovery.
func ThawFrozen() {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGCONT)
	_ = os.Remove(pidFilePath())
	slog.Info("steam: thawed frozen process on cleanup", "pid", pid)
}

// SteamInputBlocker suppresses PS/Nintendo controller input from reaching
// Steam while the overlay is visible. Two implementations:
//   - bpfBlocker: BPF LSM blocks hidraw reads (preferred, no side effects)
//   - sigstopBlocker: SIGSTOP freezes Steam (fallback, freezes games too)
type SteamInputBlocker interface {
	BlockSteam() int
	UnblockSteam(pid int)
}

// NewSteamInputBlocker creates a SteamInputBlocker. Tries BPF LSM first
// (zero side effects), falls back to SIGSTOP (freezes Steam + games).
func NewSteamInputBlocker() SteamInputBlocker {
	// shortcut: file caps (cap_bpf,cap_perfmon) stay for the process lifetime because cgo
	// blocks a process-wide capset; move the blocker into z13ctl-plus to drop setcap entirely.
	hb, err := hidblocker.New()
	if err != nil {
		slog.Info("steam: BPF LSM unavailable, using SIGSTOP fallback", "err", err)
		return &sigstopBlocker{}
	}
	slog.Info("steam: using BPF LSM hidraw blocker")
	return &bpfBlocker{hb: hb}
}

// --- BPF implementation ---

type bpfBlocker struct {
	hb *hidblocker.Blocker
}

func (b *bpfBlocker) BlockSteam() int {
	pid := FindSteamPID()
	if pid == 0 {
		return 0
	}
	if err := b.hb.Block(pid); err != nil {
		slog.Warn("steam: BPF block failed", "pid", pid, "err", err)
		return 0
	}
	slog.Info("steam: BPF blocked", "pid", pid)
	for _, child := range findDescendants(pid) {
		if err := b.hb.Block(child); err != nil {
			slog.Warn("steam: BPF block child failed", "pid", child, "err", err)
		} else {
			slog.Info("steam: BPF blocked child", "pid", child)
		}
	}
	return pid
}

func (b *bpfBlocker) UnblockSteam(pid int) {
	if pid == 0 {
		return
	}
	b.hb.UnblockAll()
	slog.Info("steam: BPF unblocked", "pid", pid)
}

// --- SIGSTOP fallback ---

type sigstopBlocker struct{}

func (s *sigstopBlocker) BlockSteam() int {
	pid := FindSteamPID()
	if pid == 0 {
		return 0
	}
	if err := FreezeProc(pid); err != nil {
		slog.Warn("steam: freeze failed", "pid", pid, "err", err)
		return 0
	}
	slog.Info("steam: frozen", "pid", pid)
	return pid
}

func (s *sigstopBlocker) UnblockSteam(pid int) {
	if pid == 0 {
		return
	}
	if err := ThawProc(pid); err != nil {
		slog.Warn("steam: thaw failed", "pid", pid, "err", err)
	} else {
		slog.Info("steam: thawed", "pid", pid)
	}
}

// findDescendants returns PIDs of all processes below root (children, games
// started through Steam's reaper, ...), from one pass over /proc.
func findDescendants(root int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	kids := map[int][]int{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + e.Name() + "/status")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "PPid:\t"); ok {
				if ppid, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					kids[ppid] = append(kids[ppid], pid)
				}
				break
			}
		}
	}
	var out []int
	for queue := kids[root]; len(queue) > 0; queue = queue[1:] {
		out = append(out, queue[0])
		queue = append(queue, kids[queue[0]]...)
	}
	return out
}
