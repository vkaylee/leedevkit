package testplan

import (
	"context"
	"fmt"

	"github.com/vkaylee/leedevkit/internal/lifecycle"
)

type ComposePhaseExecutor struct {
	Lifecycle lifecycle.Executor
	Gate      func(context.Context, Phase) PhaseResult
	UpArgs    []string
	DownArgs  []string
}

func (e ComposePhaseExecutor) Execute(phase Phase) PhaseResult {
	return e.ExecuteContext(context.Background(), phase)
}

func (e ComposePhaseExecutor) ExecuteContext(ctx context.Context, phase Phase) (result PhaseResult) {
	if phase.Status != "selected" {
		return PhaseResult{Status: phase.Status}
	}
	if e.Gate == nil {
		return PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "gate executor is required"}
	}
	result = PhaseResult{}
	err := e.Lifecycle.Run(ctx, func() error {
		result = e.Gate(ctx, phase)
		if result.Status == "passed" {
			return nil
		}
		return fmt.Errorf("phase %s: %s", phase.Name, result.Diagnostic)
	})
	if err != nil && result.Status == "" {
		result = PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: err.Error()}
	}
	return result
}
