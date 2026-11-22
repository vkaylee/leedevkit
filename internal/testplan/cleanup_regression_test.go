package testplan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/lifecycle"
)

func TestComposePhaseBlocksWhenTeardownFailsAfterSuccessfulGate(t *testing.T) {
	var commands []string
	executor := ComposePhaseExecutor{
		Lifecycle: lifecycle.Executor{
			UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
			Execute: func(_ context.Context, args []string) error {
				commands = append(commands, args[1])
				if args[1] == "down" {
					return errors.New("cleanup failed")
				}
				return nil
			},
		},
		Gate: func(context.Context, Phase) PhaseResult { return PhaseResult{Status: "passed"} },
	}
	result := executor.Execute(Phase{Name: "Unit Tests", Status: "selected"})
	if result.Status != "blocked" || !strings.Contains(result.Diagnostic, "cleanup failed") {
		t.Fatalf("result=%+v commands=%v", result, commands)
	}
}
