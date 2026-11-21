package gates

import (
	"context"
	"errors"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestExecutorRunsSelectedPhaseGates(t *testing.T) {
	var commands [][]string
	executor := Executor{Run: func(_ context.Context, request process.Request) process.Result {
		commands = append(commands, request.Args)
		return process.Result{ExitCode: 0}
	}}
	result := executor.RunPhase(context.Background(), "lint", Defaults())
	if result.Status != "passed" || len(commands) != 2 {
		t.Fatalf("result = %#v, commands = %#v", result, commands)
	}
}

func TestExecutorStopsRequiredFailure(t *testing.T) {
	calls := 0
	executor := Executor{Run: func(_ context.Context, request process.Request) process.Result {
		calls++
		if request.Args[0] == "gofmt" {
			return process.Result{ExitCode: 3, Err: errors.New("format failed")}
		}
		return process.Result{ExitCode: 0}
	}}
	result := executor.RunPhase(context.Background(), "lint", Defaults())
	if result.Status != "failed" || result.ExitCode != 3 || calls != 1 {
		t.Fatalf("result = %#v, calls = %d", result, calls)
	}
}

func TestExecutorSkipsGatesFromOtherPhases(t *testing.T) {
	calls := 0
	executor := Executor{Run: func(_ context.Context, _ process.Request) process.Result { calls++; return process.Result{ExitCode: 0} }}
	result := executor.RunPhase(context.Background(), "coverage", Defaults())
	if result.Status != "passed" || calls != 1 {
		t.Fatalf("result = %#v, calls = %d", result, calls)
	}
}

func TestExecutorRunsSetupAndTeardownAroundGate(t *testing.T) {
	var calls []string
	executor := Executor{Run: func(_ context.Context, request process.Request) process.Result {
		calls = append(calls, request.Args[0])
		return process.Result{ExitCode: 0}
	}}
	result := executor.RunPhase(context.Background(), "integration", []Gate{{Name: "integration", Phase: "integration", Command: []string{"gate"}, SetupCommand: []string{"setup"}, TeardownCommand: []string{"teardown"}}})
	if result.Status != "passed" || len(calls) != 3 || calls[0] != "setup" || calls[1] != "gate" || calls[2] != "teardown" {
		t.Fatalf("result = %#v, calls = %#v", result, calls)
	}
}

func TestExecutorRunsTeardownAfterSetupFailure(t *testing.T) {
	var calls []string
	executor := Executor{Run: func(_ context.Context, request process.Request) process.Result {
		calls = append(calls, request.Args[0])
		if request.Args[0] == "setup" {
			return process.Result{ExitCode: 7, Err: errors.New("setup failed")}
		}
		return process.Result{ExitCode: 0}
	}}
	result := executor.RunPhase(context.Background(), "integration", []Gate{{Name: "integration", Phase: "integration", Command: []string{"gate"}, SetupCommand: []string{"setup"}, TeardownCommand: []string{"teardown"}}})
	if result.Status != "failed" || len(calls) != 2 || calls[0] != "setup" || calls[1] != "teardown" {
		t.Fatalf("result = %#v, calls = %#v", result, calls)
	}
}

func TestExecutorBlocksWhenRequiredGateDidNotPass(t *testing.T) {
	executor := Executor{Run: func(context.Context, process.Request) process.Result { t.Fatal("gate ran"); return process.Result{} }, Done: map[string]bool{}}
	result := executor.RunPhase(context.Background(), "unit", []Gate{{Name: "unit", Phase: "unit", Command: []string{"unit"}, Requires: []string{"lint"}}})
	if result.Status != "blocked" || result.ExitCode == 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecutorHonorsGateTimeout(t *testing.T) {
	timeout := 0.001
	executor := Executor{Run: func(ctx context.Context, _ process.Request) process.Result {
		<-ctx.Done()
		return process.Result{ExitCode: 124, Err: ctx.Err()}
	}}
	result := executor.RunPhase(context.Background(), "unit", []Gate{{Name: "slow", Phase: "unit", Command: []string{"slow"}, Timeout: &timeout}})
	if result.Status != "failed" || result.ExitCode != 124 {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecutorRunsFixCommandWhenRequested(t *testing.T) {
	var command []string
	executor := Executor{Fix: true, Run: func(_ context.Context, request process.Request) process.Result {
		command = request.Args
		return process.Result{ExitCode: 0}
	}}
	result := executor.RunPhase(context.Background(), "lint", []Gate{{Name: "lint", Phase: "lint", Command: []string{"check"}, FixCommand: []string{"fix"}}})
	if result.Status != "passed" || len(command) != 1 || command[0] != "fix" {
		t.Fatalf("result = %#v, command = %#v", result, command)
	}
}

func TestExecutorChecksGateVersionBeforeRunningGate(t *testing.T) {
	var commands [][]string
	executor := Executor{Run: func(_ context.Context, request process.Request) process.Result {
		commands = append(commands, request.Args)
		if request.Args[0] == "tool" {
			return process.Result{ExitCode: 0, Stdout: []byte("tool 1.2.3\n")}
		}
		return process.Result{ExitCode: 0}
	}}
	result := executor.RunPhase(context.Background(), "lint", []Gate{{
		Name: "tool", Phase: "lint", Command: []string{"check"},
		VersionCommand: []string{"tool", "--version"}, ExpectedVersion: stringPtr("tool 1.2.3"),
	}})
	if result.Status != "passed" || len(commands) != 2 || commands[0][1] != "--version" {
		t.Fatalf("result = %#v, commands = %#v", result, commands)
	}
}

func TestExecutorBlocksGateWhenVersionMismatches(t *testing.T) {
	gateCalls := 0
	executor := Executor{Run: func(_ context.Context, request process.Request) process.Result {
		if request.Args[0] == "check" {
			gateCalls++
		}
		return process.Result{ExitCode: 0, Stdout: []byte("tool 9.0.0\n")}
	}}
	result := executor.RunPhase(context.Background(), "lint", []Gate{{
		Name: "tool", Phase: "lint", Command: []string{"check"},
		VersionCommand: []string{"tool", "--version"}, ExpectedVersion: stringPtr("tool 1.2"),
	}})
	if result.Status != "failed" || gateCalls != 0 || result.Diagnostic == "" {
		t.Fatalf("result = %#v, gate calls = %d", result, gateCalls)
	}
}

func TestValidateRejectsIncompleteVersionConfiguration(t *testing.T) {
	if err := Validate(Gate{Name: "tool", Phase: "lint", Command: []string{"check"}, ExpectedVersion: stringPtr("1")}); err == nil {
		t.Fatal("expected missing version command error")
	}
	if err := Validate(Gate{Name: "tool", Phase: "lint", Command: []string{"check"}, VersionCommand: []string{"tool", "--version"}}); err == nil {
		t.Fatal("expected missing expected version error")
	}
}

func stringPtr(value string) *string { return &value }
