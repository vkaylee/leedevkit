package testplan

import (
	"context"
	"errors"
	"testing"
)

func TestLifecycleExecutorStartsRunsAndTearsDown(t *testing.T) {
	calls := []string{}
	executor := LifecycleExecutor{
		Up:   func([]string) error { calls = append(calls, "up"); return nil },
		Down: func([]string) error { calls = append(calls, "down"); return nil },
		Gate: func(context.Context, Phase) PhaseResult {
			calls = append(calls, "gate")
			return PhaseResult{Status: "passed"}
		},
		UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
	}
	result := executor.Execute(context.Background(), Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "passed" || len(calls) != 3 || calls[0] != "up" || calls[1] != "gate" || calls[2] != "down" {
		t.Fatalf("result = %#v, calls = %#v", result, calls)
	}
}

func TestLifecycleExecutorBlocksWhenStartupFails(t *testing.T) {
	gateCalled := false
	executor := LifecycleExecutor{
		Up:     func([]string) error { return errors.New("startup failed") },
		Down:   func([]string) error { return nil },
		Gate:   func(context.Context, Phase) PhaseResult { gateCalled = true; return PhaseResult{Status: "passed"} },
		UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
	}
	result := executor.Execute(context.Background(), Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "blocked" || gateCalled || result.ExitCode == 0 {
		t.Fatalf("result = %#v, gateCalled = %v", result, gateCalled)
	}
}
