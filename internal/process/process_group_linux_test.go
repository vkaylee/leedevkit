//go:build linux

package process

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTimeoutStopsDescendantButNotUnrelatedProcess(t *testing.T) {
	unrelated := exec.Command("sleep", "30")
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() }()

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	var output bytes.Buffer
	result := Run(context.Background(), Request{
		Args: []string{"sh", "-c", `sleep 30 & echo $! > "$1"; wait`, "sh", pidFile},
		Env:  os.Environ(), Stdout: &output, Stderr: &output, Timeout: time.Second,
	})
	if result.ExitCode != 124 {
		t.Fatalf("timeout: %#v; output %s", result, output.String())
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	// A killed child can remain a zombie until reaped by the system init.
	if err := syscall.Kill(pid, 0); err == nil {
		state, readErr := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		if readErr != nil || !strings.Contains(string(state), ") Z ") {
			t.Fatalf("descendant %d remains running", pid)
		}
	} else if err != syscall.ESRCH {
		t.Fatal(err)
	}
	if err := unrelated.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("unrelated process was terminated: %v", err)
	}
}
