package testplan

import (
	"context"
	"fmt"
)

type LifecycleExecutor struct {
	Up       func([]string) error
	Down     func([]string) error
	Gate     func(context.Context, Phase) PhaseResult
	UpArgs   []string
	DownArgs []string
}

func (e LifecycleExecutor) Execute(ctx context.Context, phase Phase) (result PhaseResult) {
	if phase.Status != "selected" {
		return PhaseResult{Status: phase.Status}
	}
	if e.Up == nil || e.Gate == nil {
		return PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "lifecycle executor is incomplete"}
	}
	if err := e.Up(e.UpArgs); err != nil {
		return PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: fmt.Sprintf("lifecycle startup failed: %v", err)}
	}
	defer func() {
		if e.Down == nil {
			return
		}
		if err := e.Down(e.DownArgs); err != nil && result.Status == "passed" {
			result.Status = "failed"
			result.ExitCode = 1
			result.Diagnostic = fmt.Sprintf("lifecycle teardown failed: %v", err)
		}
	}()
	return e.Gate(ctx, phase)
}
