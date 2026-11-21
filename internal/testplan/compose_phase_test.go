package testplan

import (
	"context"
	"testing"

	"github.com/vkaylee/leedevkit/internal/lifecycle"
)

func TestComposePhaseExecutorRunsBuildBeforeLifecycleAndGate(t *testing.T) {
	calls := []string{}
	executor := ComposePhaseExecutor{
		Build: func(context.Context, Phase) error {
			calls = append(calls, "build")
			return nil
		},
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
	if result.Status != "passed" || len(calls) != 4 || calls[0] != "build" || calls[1] != "up" || calls[2] != "gate" || calls[3] != "down" {
		t.Fatalf("result = %#v, calls = %#v", result, calls)
	}
}

func TestComposePhaseExecutorBlocksWhenBuildFails(t *testing.T) {
	executor := ComposePhaseExecutor{
		Build: func(context.Context, Phase) error { return context.Canceled },
		Lifecycle: lifecycle.Executor{
			UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
			Execute: func(context.Context, []string) error { t.Fatal("lifecycle started after build failure"); return nil },
		},
		Gate: func(context.Context, Phase) PhaseResult {
			t.Fatal("gate ran after build failure")
			return PhaseResult{}
		},
	}
	result := executor.ExecuteContext(context.Background(), Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "blocked" || result.ExitCode == 0 {
		t.Fatalf("result = %#v", result)
	}
}
