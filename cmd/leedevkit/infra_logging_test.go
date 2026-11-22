package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

func TestRunInfraLogsEveryCheckAndPrefixesLiveOutput(t *testing.T) {
	root := t.TempDir()
	var live strings.Builder
	var seen int
	result := runInfra(root, testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		seen++
		if request.Timeout <= 0 {
			t.Error("child check has no timeout")
		}
		_, _ = request.Stdout.Write([]byte("line one\nlast line"))
		return process.Result{}
	}, &live)
	if seen != 12 {
		t.Fatalf("checks = %d, want all 12 checks", seen)
	}
	if !strings.Contains(live.String(), "[infra-01] line one\n") || !strings.Contains(live.String(), "[infra-01] last line") {
		t.Fatalf("live output lacks task prefixes or altered unterminated output: %q", live.String())
	}
	for i := 1; i <= seen; i++ {
		path := filepath.Join(root, ".test_logs")
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("run dirs = %d, want one isolated run dir", len(entries))
		}
		logPath := filepath.Join(path, entries[0].Name(), infraLogName(i))
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "line one\nlast line" {
			t.Fatalf("task log %s = %q", logPath, data)
		}
	}
	if result.Status != "passed" || result.ExitCode != 0 {
		t.Fatalf("result = %#v; completed infrastructure checks must pass", result)
	}
}

func TestRunInfraSuppressesLiveOutputForMachineStreamButKeepsLog(t *testing.T) {
	root := t.TempDir()
	var live strings.Builder
	result := runInfra(root, testplan.Flags{Target: "infra", JSONStream: true}, func(_ context.Context, request process.Request) process.Result {
		_, _ = request.Stdout.Write([]byte("secret diagnostic-free output"))
		return process.Result{}
	}, &live)
	if strings.Contains(live.String(), "secret diagnostic-free output") {
		t.Fatalf("machine stream contains child output: %q", live.String())
	}
	if !strings.Contains(live.String(), `"event":"run_started"`) || !strings.Contains(live.String(), `"event":"run_finished"`) {
		t.Fatalf("machine stream lacks valid run events: %q", live.String())
	}
	entries, err := os.ReadDir(filepath.Join(root, ".test_logs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("log run dirs = %v, err = %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".test_logs", entries[0].Name(), infraLogName(1)))
	if err != nil || string(data) != "secret diagnostic-free output" {
		t.Fatalf("quiet log = %q, err = %v", data, err)
	}
	if result.Status != "passed" {
		t.Fatalf("status = %q, want passed", result.Status)
	}
	finished := false
	for _, line := range strings.Split(strings.TrimSpace(live.String()), "\n") {
		var event struct {
			Event   string `json:"event"`
			Status  string `json:"status"`
			Full    bool   `json:"full_regression"`
			Summary struct {
				Exit int `json:"exit_code"`
			} `json:"summary"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event == "run_finished" {
			finished = true
			if event.Status != "passed" || !event.Full || event.Summary.Exit != 0 {
				t.Fatalf("incorrect final event: %s", line)
			}
		}
	}
	if !finished {
		t.Fatal("missing run_finished event")
	}
}

func TestRunInfraFailureIncludesTaskLogAndContinuesAllChecks(t *testing.T) {
	root := t.TempDir()
	var calls int
	result := runInfra(root, testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		calls++
		_, _ = request.Stderr.Write([]byte("check failed"))
		if calls == 3 {
			return process.Result{ExitCode: 7, Stderr: []byte("check failed")}
		}
		return process.Result{}
	}, nil)
	if calls != 12 {
		t.Fatalf("checks = %d, want all 12 despite failure", calls)
	}
	if result.Status != "failed" || result.ExitCode != 7 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Failed) != 1 || !strings.HasSuffix(result.Failed[0].Log, "/infra-03.log") {
		t.Fatalf("failed phases lack machine-readable log path: %#v", result.Failed)
	}
	if !strings.Contains(result.Failed[0].Diagnostic, "infra-03.log") {
		t.Fatalf("failed phases lack log location: %#v", result.Failed)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".test_logs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("log run dirs = %v, err = %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".test_logs", entries[0].Name(), infraLogName(3)))
	if err != nil || !strings.Contains(string(data), "check failed") {
		t.Fatalf("failed task log = %q, err = %v", data, err)
	}
}

func TestRunInfraFailsClosedWhenLogRootIsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".test_logs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	var calls int
	result := runInfra(root, testplan.Flags{Target: "infra"}, func(context.Context, process.Request) process.Result {
		calls++
		return process.Result{}
	}, nil)
	if calls != 0 || result.Status != "blocked" || result.ExitCode == 0 {
		t.Fatalf("calls=%d result=%#v; unsafe log path must block before checks", calls, result)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside log directory changed: entries=%v err=%v", entries, err)
	}
}

func TestRunInfraQuietKeepsStdoutAndStderrInLog(t *testing.T) {
	root := t.TempDir()
	var live strings.Builder
	result := runInfra(root, testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		_, _ = request.Stdout.Write([]byte("stdout\n"))
		_, _ = request.Stderr.Write([]byte("stderr"))
		return process.Result{}
	}, &live, true)
	if live.Len() != 0 {
		t.Fatalf("quiet emitted output: %q", live.String())
	}
	entries, err := os.ReadDir(filepath.Join(root, ".test_logs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("log directories = %v, err = %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".test_logs", entries[0].Name(), infraLogName(1)))
	if err != nil || string(data) != "stdout\nstderr" {
		t.Fatalf("log = %q, err = %v", data, err)
	}
	if result.Status != "passed" || result.ExitCode != 0 {
		t.Fatalf("result = %#v", result)
	}
}

type failingInfraWriter struct{}

func (failingInfraWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestInfraOutputWriterReportsIgnoredWriteErrors(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	writer := &infraOutputWriter{log: file, live: failingInfraWriter{}}
	_, _ = writer.Write([]byte("lost output"))
	if err := writer.Close(); err == nil {
		t.Fatal("ignored write error must not be reported as success")
	}
}

func TestRunInfraFailsWhenLiveOutputWriteFails(t *testing.T) {
	result := runInfra(t.TempDir(), testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		_, _ = request.Stdout.Write([]byte("output"))
		return process.Result{}
	}, failingInfraWriter{})
	if result.Status != "failed" || result.ExitCode == 0 || len(result.Failed) != 1 || !strings.Contains(result.Failed[0].Diagnostic, "output write failed") {
		t.Fatalf("write failure not reported: %#v", result)
	}
}

func TestRunInfraPassesBoundedChildTimeout(t *testing.T) {
	root := t.TempDir()
	var timeout time.Duration
	runInfra(root, testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		timeout = request.Timeout
		return process.Result{ExitCode: 124}
	}, nil)
	if timeout <= 0 || timeout > 30*time.Minute {
		t.Fatalf("timeout = %s, want (0, 30m]", timeout)
	}
}
