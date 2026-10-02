//go:build !windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func startProcessGroup(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, args...)
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	go func() { _ = cmd.Wait() }()
	return cmd
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitProcessGoneForTest polls until the process is reaped by the Wait
// goroutine; SIGKILL is asynchronous with respect to reaping.
func waitProcessGoneForTest(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for ProcessExists(pid) {
		if !time.Now().Before(deadline) {
			t.Fatalf("process %d did not exit", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestKillTreeStopsOnTerm(t *testing.T) {
	cmd := startProcessGroup(t, "sleep", "30")
	pid := cmd.Process.Pid

	start := time.Now()
	if err := killTree(pid); err != nil {
		t.Fatalf("killTree: %v", err)
	}
	elapsed := time.Since(start)

	waitProcessGoneForTest(t, pid)
	if elapsed >= stopWaitLimit {
		t.Fatalf("process was not stopped by SIGTERM within the limit (took %v)", elapsed)
	}
}

func TestKillTreeEscalatesToKill(t *testing.T) {
	// The process sets SIGTERM to SIG_IGN and execs sleep, so it keeps the
	// ignored disposition and survives the initial group SIGTERM. It touches
	// the marker first so the test only signals once the ignore is in place.
	marker := filepath.Join(t.TempDir(), "ready")
	cmd := startProcessGroup(t, "bash", "-c",
		`trap "" TERM; touch "$1"; exec sleep 30`, "_", marker)
	pid := cmd.Process.Pid

	waitForFile(t, marker)

	start := time.Now()
	if err := killTree(pid); err != nil {
		t.Fatalf("killTree: %v", err)
	}
	elapsed := time.Since(start)

	waitProcessGoneForTest(t, pid)
	if elapsed < stopWaitLimit {
		t.Fatalf("expected SIGKILL escalation after the wait limit, took %v", elapsed)
	}
}

func TestKillTreeDeadProcessIsNoop(t *testing.T) {
	cmd := startProcessGroup(t, "sleep", "30")
	pid := cmd.Process.Pid

	if err := killTree(pid); err != nil {
		t.Fatalf("killTree: %v", err)
	}
	if err := killTree(pid); err != nil {
		t.Fatalf("killTree on an already dead pid should be a no-op, got %v", err)
	}
}
