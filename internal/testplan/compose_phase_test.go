package testplan

import (
	"context"
	"strings"
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

func TestComposePhaseExecutorReportsLifecycleFailureDetails(t *testing.T) {
	executor := ComposePhaseExecutor{
		Lifecycle: lifecycle.Executor{
			UpArgs:   []string{"docker", "compose", "-f", "/home/test/project/.compose/docker-compose.test.yml", "up", "-d", "go"},
			DownArgs: []string{"docker", "compose", "down"},
			Execute: func(_ context.Context, args []string) error {
				for _, arg := range args {
					if arg == "up" {
						return &lifecycle.CommandError{ExitCode: 1, Stderr: []byte("no such service api\n")}
					}
				}
				return nil
			},
		},
		Gate: func(context.Context, Phase) PhaseResult {
			t.Fatal("gate ran after lifecycle failure")
			return PhaseResult{}
		},
	}
	result := executor.ExecuteContext(context.Background(), Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "blocked" || result.ExitCode != 1 {
		t.Fatalf("result = %#v", result)
	}
	if result.Command == "" || !strings.Contains(result.Command, "docker compose") || strings.Contains(result.Command, "/home/test") {
		t.Fatalf("command = %q", result.Command)
	}
	if !strings.Contains(result.Diagnostic, "no such service api") || !strings.Contains(result.Diagnostic, "startup") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if !strings.Contains(result.LifecycleError, "no such service api") {
		t.Fatalf("lifecycle error = %q", result.LifecycleError)
	}
}

func TestRunContextPropagatesContextToExecutor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var sawCanceled bool
	result := RunContext(ctx, Flags{Target: "go", UnitOnly: true}, ContextExecutorFunc(func(phaseCtx context.Context, _ Phase) PhaseResult {
		sawCanceled = phaseCtx.Err() != nil
		return PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "canceled"}
	}), nil)
	if !sawCanceled || result.Status != "blocked" {
		t.Fatalf("sawCanceled=%v result=%#v", sawCanceled, result)
	}
}
