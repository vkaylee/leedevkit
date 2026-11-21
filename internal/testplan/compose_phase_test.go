package testplan

import (
	"context"
	"testing"

	"github.com/vkaylee/leedevkit/internal/lifecycle"
)

func TestComposePhaseExecutorRunsLifecycleAndGate(t *testing.T) {
	calls := []string{}
	executor := ComposePhaseExecutor{
		Lifecycle: lifecycle.Executor{
			UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
			Execute: func(_ context.Context, args []string) error { calls = append(calls, args[1]); return nil },
		},
		Gate: func(context.Context, Phase) PhaseResult {
			calls = append(calls, "gate")
			return PhaseResult{Status: "passed"}
		},
	}
	result := executor.ExecuteContext(context.Background(), Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "passed" || len(calls) != 3 || calls[0] != "up" || calls[1] != "gate" || calls[2] != "down" {
		t.Fatalf("result = %#v, calls = %#v", result, calls)
	}
}
