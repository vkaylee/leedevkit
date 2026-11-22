package testplan

import (
	"context"
	"strings"
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
}
