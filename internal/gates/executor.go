package gates

import (
	"context"
	"path/filepath"

	"github.com/vkaylee/leedevkit/internal/process"
)

type Executor struct {
	Run func(context.Context, process.Request) process.Result
}

type PhaseResult struct {
	Status     string
	ExitCode   int
	Diagnostic string
	Gate       string
}

func (e Executor) RunPhase(ctx context.Context, phase string, definitions []Gate) PhaseResult {
	run := e.Run
	if run == nil {
		run = process.Run
	}
	for _, gate := range definitions {
		if gate.Phase != phase {
			continue
		}
		command := gate.Command
		if gate.Mode == "check" && gate.Name == "gofmt" {
			command = []string{"gofmt", "-l", "."}
		}
		result := run(ctx, process.Request{Args: command, Dir: filepath.Clean(gate.WorkingDir), Env: gate.Environment})
		if result.ExitCode != 0 {
			status := "failed"
			if gate.FailurePolicy == "optional" && !gate.Required {
				status = "passed"
			}
			return PhaseResult{Status: status, ExitCode: result.ExitCode, Diagnostic: errorText(result.Err), Gate: gate.Name}
		}
	}
	return PhaseResult{Status: "passed"}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
