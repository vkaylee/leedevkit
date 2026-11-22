package testplan

import (
	"context"
	"fmt"
	"strings"

	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/process"
)

type ComposePhaseExecutor struct {
	Build     func(context.Context, Phase) error
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
		return PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "gate executor is required", Command: strings.Join(process.RedactArgs(e.Lifecycle.UpArgs), " ")}
	}
	if e.Build != nil {
		if err := e.Build(ctx, phase); err != nil {
			return PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: fmt.Sprintf("build: %v", err), Command: strings.Join(process.RedactArgs(e.Lifecycle.UpArgs), " ")}
		}
	}
	result = PhaseResult{}
	err := e.Lifecycle.Run(ctx, func() error {
		result = e.Gate(ctx, phase)
		if result.Status == "passed" {
			return nil
		}
		return fmt.Errorf("phase %s: %s", phase.Name, result.Diagnostic)
	})
	if err != nil && (result.Status == "" || result.Status == "passed") {
		command := e.Lifecycle.UpArgs
		if failure, ok := err.(*lifecycle.Failure); ok {
			command = failure.Args
		}
		result = PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: process.RedactDiagnostic(err.Error()), Command: strings.Join(process.RedactArgs(command), " ")}
	}
	return result
}
