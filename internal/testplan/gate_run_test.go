package testplan

import (
	"context"
	"testing"

	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/process"
)

func TestGatePhaseExecutorPropagatesBlockedGatePolicy(t *testing.T) {
	executor := GatePhaseExecutor{
		Definitions: []gates.Gate{{Name: "required-service", Phase: "unit", Command: []string{"check"}, FailurePolicy: "block", Required: true}},
		Run: func(context.Context, process.Request) process.Result {
			return process.Result{ExitCode: 9}
		},
	}
	result := executor.Execute(Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "blocked" || result.ExitCode != 9 {
		t.Fatalf("result = %#v", result)
	}
}

func TestGatePhaseExecutorMapsSchedulerPhase(t *testing.T) {
	calls := 0
	executor := GatePhaseExecutor{
		Definitions: gates.Defaults(),
		Run:         func(_ context.Context, _ process.Request) process.Result { calls++; return process.Result{ExitCode: 0} },
	}
	result := executor.Execute(Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "passed" || calls != 1 {
		t.Fatalf("result = %#v, calls = %d", result, calls)
	}
}
