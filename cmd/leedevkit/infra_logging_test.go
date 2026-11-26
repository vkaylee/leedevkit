package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		if request.Timeout <= 0 {
			t.Error("child check has no timeout")
		}
		if hasArg(request.Args, "exec") {
			seen++
			_, _ = request.Stdout.Write([]byte("line one\nlast line"))
		}
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
		if !hasArg(request.Args, "exec") {
			return process.Result{}
		}
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

func TestRunInfraStreamIncludesParsedFailure(t *testing.T) {
	root := t.TempDir()
	var stream strings.Builder
	calls := 0
	runInfra(root, testplan.Flags{Target: "infra", JSONStream: true}, func(_ context.Context, request process.Request) process.Result {
		if !hasArg(request.Args, "exec") {
			return process.Result{}
		}
		calls++
		if calls == 1 {
			_, _ = request.Stdout.Write([]byte("--- FAIL: TestBroken (0.00s)\n    bad_test.go:9: broken\n"))
			return process.Result{ExitCode: 1}
		}
		return process.Result{}
	}, &stream)
	lines := strings.Split(strings.TrimSpace(stream.String()), "\n")
	var event struct {
		Summary struct {
			Failures []struct {
				TestName string `json:"test_name"`
			} `json:"failures"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatal(err)
	}
	if len(event.Summary.Failures) != 1 || event.Summary.Failures[0].TestName != "TestBroken" {
		t.Fatalf("event = %#v", event)
	}
}

func TestRunInfraKeepsAllFailedCheckLogs(t *testing.T) {
	root := t.TempDir()
	calls := 0
	result := runInfra(root, testplan.Flags{Target: "infra", Compact: true}, func(_ context.Context, request process.Request) process.Result {
		if !hasArg(request.Args, "exec") {
			return process.Result{}
		}
		calls++
		_, _ = request.Stdout.Write([]byte("check output"))
		if calls == 2 || calls == 4 {
			return process.Result{ExitCode: 1}
		}
		return process.Result{}
	}, nil)
	if len(result.FailureLogs) != 2 || !strings.HasSuffix(result.FailureLogs[0], "infra-02.log") || !strings.HasSuffix(result.FailureLogs[1], "infra-04.log") {
		t.Fatalf("failed logs = %v", result.FailureLogs)
	}
}

func TestRunInfraStartupFailureBlocksAndCleansUp(t *testing.T) {
	root := t.TempDir()
	var ops []string
	result := runInfra(root, testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		switch {
		case hasArg(request.Args, "up"):
			ops = append(ops, "up")
			return process.Result{ExitCode: 1, Stderr: []byte("daemon unavailable")}
		case hasArg(request.Args, "down"):
			ops = append(ops, "down")
			return process.Result{}
		default:
			ops = append(ops, "exec")
			return process.Result{}
		}
	}, nil)
	if result.Status != "blocked" || result.ExitCode != 1 {
		t.Fatalf("result = %#v, want blocked exit 1", result)
	}
	if strings.Join(ops, ",") != "up,down" {
		t.Fatalf("ops = %v, want [up down]", ops)
	}
}

func TestRunInfraTeardownFailureFailsPassingRun(t *testing.T) {
	root := t.TempDir()
	result := runInfra(root, testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		if hasArg(request.Args, "down") {
			return process.Result{ExitCode: 1, Stderr: []byte("down failed")}
		}
		return process.Result{}
	}, nil)
	if result.Status != "failed" || result.ExitCode != 1 || !strings.Contains(result.LifecycleError, "teardown") {
		t.Fatalf("result = %#v, want teardown failure", result)
	}
	if len(result.Failed) != 1 || !strings.Contains(result.Failed[0].Command, "down --remove-orphans") || strings.Contains(result.Failed[0].Command, "up -d") {
		t.Fatalf("failed phase command = %#v, want down command", result.Failed)
	}
}

func TestRunInfraCanceledContextStopsAndRunsDown(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	var ops []string
	var downCtxErr error
	result := runInfraContext(ctx, root, testplan.Flags{Target: "infra"}, func(callCtx context.Context, request process.Request) process.Result {
		switch {
		case hasArg(request.Args, "up"):
			ops = append(ops, "up")
			return process.Result{}
		case hasArg(request.Args, "down"):
			ops = append(ops, "down")
			downCtxErr = callCtx.Err()
			return process.Result{}
		default:
			ops = append(ops, "exec")
			cancel()
			return process.Result{ExitCode: 1, Err: context.Canceled}
		}
	}, nil)
	if result.Status != "failed" || result.ExitCode != 1 {
		t.Fatalf("result = %#v, want failed exit 1", result)
	}
	if strings.Join(ops, ",") != "up,exec,down" {
		t.Fatalf("ops = %v, want [up exec down]", ops)
	}
	if downCtxErr != nil {
		t.Fatalf("down context was canceled: %v", downCtxErr)
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

func TestRunInfraCompactUsesGoTestJSONAndShowsDirectError(t *testing.T) {
	root := t.TempDir()
	var compactOut strings.Builder
	var sawGoTestJSON bool
	result := runInfra(root, testplan.Flags{Target: "infra", Compact: true, UnitOnly: true}, func(_ context.Context, request process.Request) process.Result {
		if !hasArg(request.Args, "exec") {
			return process.Result{}
		}
		joined := strings.Join(request.Args, " ")
		if strings.Contains(joined, "go test -json ./...") {
			sawGoTestJSON = true
			_, _ = request.Stdout.Write([]byte(
				`{"Action":"run","Package":"example/p","Test":"TestBroken"}` + "\n" +
					`{"Action":"output","Package":"example/p","Test":"TestBroken","Output":"broken_test.go:17: expected 2 got 1\n"}` + "\n" +
					`{"Action":"fail","Package":"example/p","Test":"TestBroken"}` + "\n" +
					`{"Action":"fail","Package":"example/p"}` + "\n",
			))
			return process.Result{ExitCode: 1}
		}
		return process.Result{}
	}, &compactOut)
	if !sawGoTestJSON {
		t.Fatal("compact infra run did not invoke go test -json ./...")
	}
	if result.Status != "failed" || result.ExitCode != 1 {
		t.Fatalf("result = %#v", result)
	}
	out := compactOut.String()
	if strings.Contains(out, "Cannot filter") {
		t.Fatalf("compact output fell back to Cannot filter: %q", out)
	}
	if !strings.Contains(out, "broken_test.go:17: expected 2 got 1") {
		t.Fatalf("compact output missing direct failure: %q", out)
	}
	if len(result.Failures) != 1 || result.Failures[0].TestName != "TestBroken" {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestRunInfraCompactMultipleFailuresDoNotRepeatMegaDiagnostic(t *testing.T) {
	root := t.TempDir()
	var compactOut strings.Builder
	result := runInfra(root, testplan.Flags{Target: "infra", Compact: true, LintOnly: true}, func(_ context.Context, request process.Request) process.Result {
		if !hasArg(request.Args, "exec") {
			return process.Result{}
		}
		joined := strings.Join(request.Args, " ")
		if strings.Contains(joined, "gofmt -l") {
			_, _ = request.Stdout.Write([]byte("unformatted Go files:\ncmd/leedevkit/infra.go\n"))
			return process.Result{ExitCode: 1}
		}
		if strings.Contains(joined, "go vet ./...") {
			_, _ = request.Stderr.Write([]byte("./cmd/leedevkit/infra.go:42:2: unreachable code\n"))
			return process.Result{ExitCode: 1}
		}
		return process.Result{}
	}, &compactOut)
	if result.Status != "failed" || len(result.FailureLogs) != 2 {
		t.Fatalf("result = %#v", result)
	}
	out := compactOut.String()
	if strings.Contains(out, "Cannot filter") {
		t.Fatalf("compact output fell back to Cannot filter: %q", out)
	}
	if strings.Count(out, "infra-01.log") != 1 || strings.Count(out, "infra-02.log") != 1 {
		t.Fatalf("expected each failed log path once, got: %q", out)
	}
	if !strings.Contains(out, "unformatted Go files:") || !strings.Contains(out, "unreachable code") {
		t.Fatalf("compact output missing concise error excerpts: %q", out)
	}
	if len(result.Failures) != 2 {
		t.Fatalf("failures = %#v, want 2 per-check failures", result.Failures)
	}
}

func TestRunInfraFocusedLintOnlyAndUnitOnly(t *testing.T) {
	t.Run("lint-only", func(t *testing.T) {
		var cmds []string
		result := runInfra(t.TempDir(), testplan.Flags{Target: "infra", LintOnly: true, Compact: true}, func(_ context.Context, request process.Request) process.Result {
			if hasArg(request.Args, "exec") {
				cmds = append(cmds, strings.Join(request.Args, " "))
			}
			return process.Result{}
		}, nil)
		if result.Status != "partial" || result.FullRegression || result.ExitCode != 0 {
			t.Fatalf("lint-only result = %#v, want partial/non-full/exit 0", result)
		}
		if len(result.Phases) != 1 || result.Phases[0].Phase != "Linting" {
			t.Fatalf("phases = %#v, want Linting", result.Phases)
		}
		if len(cmds) != 8 {
			t.Fatalf("lint-only ran %d checks, want 8: %v", len(cmds), cmds)
		}
		joined := strings.Join(cmds, "\n")
		if strings.Contains(joined, "go test") || strings.Contains(joined, "bash tests/install-test.sh") {
			t.Fatalf("lint-only unexpectedly ran test/script checks:\n%s", joined)
		}
	})

	t.Run("unit-only", func(t *testing.T) {
		var cmds []string
		result := runInfra(t.TempDir(), testplan.Flags{Target: "infra", UnitOnly: true, Compact: true}, func(_ context.Context, request process.Request) process.Result {
			if hasArg(request.Args, "exec") {
				cmds = append(cmds, strings.Join(request.Args, " "))
			}
			return process.Result{}
		}, nil)
		if result.Status != "partial" || result.FullRegression || result.ExitCode != 0 {
			t.Fatalf("unit-only result = %#v, want partial/non-full/exit 0", result)
		}
		if len(result.Phases) != 1 || result.Phases[0].Phase != "Unit Tests" {
			t.Fatalf("phases = %#v, want Unit Tests", result.Phases)
		}
		if len(cmds) != 1 || !strings.Contains(cmds[0], "go test -json ./...") {
			t.Fatalf("unit-only cmds = %v, want single go test -json", cmds)
		}
	})
}

func TestRunInfraFailFastOnGoBuildFailureSkipsVetAndCoverage(t *testing.T) {
	root := t.TempDir()
	var cmds []string
	var compactOut strings.Builder
	result := runInfra(root, testplan.Flags{Target: "infra", Compact: true}, func(_ context.Context, request process.Request) process.Result {
		if !hasArg(request.Args, "exec") {
			return process.Result{}
		}
		joined := strings.Join(request.Args, " ")
		cmds = append(cmds, joined)
		if strings.Contains(joined, "go test -json ./...") {
			_, _ = request.Stdout.Write([]byte(
				`{"Action":"build-output","ImportPath":"github.com/vkaylee/leedevkit/cmd/leedevkit","Output":"# github.com/vkaylee/leedevkit/cmd/leedevkit\ncmd/leedevkit/bad.go:10:5: undefined: missing\n"}` + "\n" +
					`{"Action":"build-fail","ImportPath":"github.com/vkaylee/leedevkit/cmd/leedevkit"}` + "\n" +
					`{"Action":"fail","Package":"github.com/vkaylee/leedevkit/cmd/leedevkit","FailedBuild":"github.com/vkaylee/leedevkit/cmd/leedevkit"}` + "\n",
			))
			return process.Result{ExitCode: 1}
		}
		return process.Result{}
	}, &compactOut)
	if result.Status != "failed" || result.ExitCode != 1 {
		t.Fatalf("result = %#v", result)
	}
	joined := strings.Join(cmds, "\n")
	if strings.Contains(joined, "go vet ./...") || strings.Contains(joined, "go test -coverpkg=./...") {
		t.Fatalf("compile failure should skip go vet and coverage, ran:\n%s", joined)
	}
	if !strings.Contains(joined, "gofmt -l") || !strings.Contains(joined, "bash -n install.sh") {
		t.Fatalf("compile failure must still run independent checks, ran:\n%s", joined)
	}
	if len(cmds) != 10 {
		t.Fatalf("ran %d checks, want 10 (12 - vet - coverage)", len(cmds))
	}
	if !strings.Contains(compactOut.String(), "undefined: missing") {
		t.Fatalf("compact output missing compile error: %q", compactOut.String())
	}
}

func TestRunInfraFailFastOnGoUnitFailureSkipsCoverageOnly(t *testing.T) {
	var cmds []string
	result := runInfra(t.TempDir(), testplan.Flags{Target: "infra", Compact: true}, func(_ context.Context, request process.Request) process.Result {
		if !hasArg(request.Args, "exec") {
			return process.Result{}
		}
		joined := strings.Join(request.Args, " ")
		cmds = append(cmds, joined)
		if strings.Contains(joined, "go test -json ./...") {
			_, _ = request.Stdout.Write([]byte(
				`{"Action":"output","Package":"p","Test":"TestFail","Output":"x_test.go:3: boom\n"}` + "\n" +
					`{"Action":"fail","Package":"p","Test":"TestFail"}` + "\n" +
					`{"Action":"fail","Package":"p"}` + "\n",
			))
			return process.Result{ExitCode: 1}
		}
		return process.Result{}
	}, nil)
	if result.Status != "failed" {
		t.Fatalf("result = %#v", result)
	}
	joined := strings.Join(cmds, "\n")
	if !strings.Contains(joined, "go vet ./...") {
		t.Fatalf("non-build unit failure should still run go vet:\n%s", joined)
	}
	if strings.Contains(joined, "go test -coverpkg=./...") {
		t.Fatalf("unit test failure should skip redundant coverage run:\n%s", joined)
	}
	if len(cmds) != 11 {
		t.Fatalf("ran %d checks, want 11 (12 - coverage)", len(cmds))
	}
}

func TestCreateInfraLogDirPrunesOldDirsSafely(t *testing.T) {
	root := t.TempDir()
	logsRoot := filepath.Join(root, ".test_logs")
	if err := os.MkdirAll(logsRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	symlinkPath := filepath.Join(logsRoot, "infra-symlink")
	if err := os.Symlink(outside, symlinkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	otherDir := filepath.Join(logsRoot, "coverage-keep")
	if err := os.Mkdir(otherDir, 0o700); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(logsRoot, "infra-file.log")
	if err := os.WriteFile(otherFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	for i := 0; i < maxInfraLogDirs+5; i++ {
		dir := filepath.Join(logsRoot, fmt.Sprintf("infra-old-%02d", i))
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		ts := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(dir, ts, ts); err != nil {
			t.Fatal(err)
		}
	}

	newest, err := createInfraLogDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newest); err != nil {
		t.Fatalf("newest log dir removed: %v", err)
	}
	if info, err := os.Lstat(symlinkPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink was altered or deleted: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(otherDir); err != nil {
		t.Fatalf("non-infra dir removed: %v", err)
	}
	if _, err := os.Stat(otherFile); err != nil {
		t.Fatalf("non-dir file removed: %v", err)
	}
	entries, err := os.ReadDir(logsRoot)
	if err != nil {
		t.Fatal(err)
	}
	infraDirCount := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "infra-") {
			continue
		}
		info, err := os.Lstat(filepath.Join(logsRoot, entry.Name()))
		if err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			infraDirCount++
		}
	}
	if infraDirCount != maxInfraLogDirs {
		t.Fatalf("retained %d infra-* dirs, want %d", infraDirCount, maxInfraLogDirs)
	}
}
