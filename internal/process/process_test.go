package process

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunForwardsSeparateStreamsAndExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := Run(context.Background(), Request{
		Args:   []string{os.Args[0], "-test.run=TestProcessHelper", "--", "output"},
		Env:    []string{"GO_WANT_PROCESS_HELPER=1"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7 (error=%v)", result.ExitCode, result.Err)
	}
	if stdout.String() != "stdout\n" || stderr.String() != "stderr\n" {
		t.Fatalf("streams = %q / %q", stdout.String(), stderr.String())
	}
}

func TestRunMapsMissingExecutableTo127(t *testing.T) {
	result := Run(context.Background(), Request{Args: []string{"/no/such/leedevkit-child"}})
	if result.ExitCode != 127 {
		t.Fatalf("exit code = %d, want 127", result.ExitCode)
	}
	if result.Err == nil || !strings.Contains(result.Err.Error(), "executable") {
		t.Fatalf("error = %v, want executable error", result.Err)
	}
}

func TestRunMapsTimeoutTo124AndKillsProcessGroup(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := Run(context.Background(), Request{
		Args:    []string{os.Args[0], "-test.run=TestProcessHelper", "--", "sleep"},
		Env:     []string{"GO_WANT_PROCESS_HELPER=1"},
		Stdout:  &stdout,
		Stderr:  &stderr,
		Timeout: 50 * time.Millisecond,
	})
	if result.ExitCode != 124 {
		t.Fatalf("exit code = %d, want 124 (error=%v)", result.ExitCode, result.Err)
	}
}

func TestRunMapsCancellationTo130(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := Run(ctx, Request{Args: []string{os.Args[0], "-test.run=TestProcessHelper", "--", "sleep"}})
	if result.ExitCode != 130 {
		t.Fatalf("exit code = %d, want 130", result.ExitCode)
	}
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROCESS_HELPER") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	switch os.Args[separator+1] {
	case "output":
		os.Stdout.WriteString("stdout\n")
		os.Stderr.WriteString("stderr\n")
		os.Exit(7)
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	default:
		os.Exit(2)
	}
}
