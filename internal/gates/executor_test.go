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
