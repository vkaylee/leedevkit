package testplan

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/report"
)

type PhaseResult struct {
	Status     string
	ExitCode   int
	Diagnostic string
}

type Executor interface {
	Execute(Phase) PhaseResult
}

type ContextExecutor interface {
	ExecuteContext(context.Context, Phase) PhaseResult
}

type ExecutorFunc func(Phase) PhaseResult

type GatePhaseExecutor struct {
	Definitions []gates.Gate
	Run         func(context.Context, process.Request) process.Result
	ComposeRun  func(context.Context, []string, []string) process.Result
}

func (e GatePhaseExecutor) Execute(phase Phase) PhaseResult {
	return e.ExecuteContext(context.Background(), phase)
}

func (e GatePhaseExecutor) ExecuteContext(ctx context.Context, phase Phase) PhaseResult {
	run := e.Run
	if e.ComposeRun != nil {
		run = func(ctx context.Context, request process.Request) process.Result {
			return e.ComposeRun(ctx, request.Args, request.Env)
		}
	}
	result := gates.Executor{Run: run}.RunPhase(ctx, phaseName(phase.Name), e.Definitions)
	return PhaseResult{Status: result.Status, ExitCode: result.ExitCode, Diagnostic: result.Diagnostic}
}

func phaseName(name string) string {
	switch name {
	case "Unit Tests":
		return "unit"
	case "Integration Tests":
		return "integration"
	case "Linting":
		return "lint"
	case "Coverage":
		return "coverage"
	default:
		return name
	}
}

func (f ExecutorFunc) Execute(phase Phase) PhaseResult { return f(phase) }

type RunResult struct {
	Status         string
	FullRegression bool
	ExitCode       int
	Counts         report.Counts
	Phases         []report.Phase
	Failed         []report.Phase
	Blocked        []report.Phase
	Skipped        []report.Phase
}

func Run(flags Flags, executor Executor, streamWriter io.Writer) RunResult {
	started := time.Now()
	phases, err := Schedule(flags)
	if err != nil {
		phase := report.Phase{Phase: "Validation", Target: flags.Target, Status: "blocked", Required: true, Diagnostic: err.Error()}
		return RunResult{Status: "blocked", FullRegression: false, ExitCode: 2, Counts: report.Counts{Blocked: 1}, Phases: []report.Phase{phase}, Blocked: []report.Phase{phase}}
	}
	runID := fmt.Sprintf("go-%d", started.UnixNano())
	var stream *report.Stream
	if streamWriter != nil && flags.JSONStream {
		stream, _ = report.NewStream(streamWriter, runID)
		_ = stream.Emit("run_started", map[string]any{"target": flags.Target})
	}
	result := RunResult{}
	for _, phase := range phases {
		record := report.Phase{Phase: phase.Name, Target: flags.Target, Status: phase.Status, Required: true}
		if phase.Status == "selected" {
			if executor == nil {
				record.Status = "blocked"
				record.Diagnostic = "phase executor is required"
			} else {
				var outcome PhaseResult
				if contextual, ok := executor.(ContextExecutor); ok {
					outcome = contextual.ExecuteContext(context.Background(), phase)
				} else {
					outcome = executor.Execute(phase)
				}
				record.Status = outcome.Status
				record.Diagnostic = outcome.Diagnostic
				code := outcome.ExitCode
				record.ExitCode = &code
			}
		}
		result.Phases = append(result.Phases, record)
		if record.Status == "passed" {
			result.Counts.Passed++
		}
		if record.Status == "failed" {
			result.Counts.Failed++
			result.Failed = append(result.Failed, record)
		}
		if record.Status == "blocked" {
			result.Counts.Blocked++
			result.Blocked = append(result.Blocked, record)
		}
		if record.Status == "skipped" {
			result.Counts.Skipped++
			result.Skipped = append(result.Skipped, record)
		}
		if stream != nil {
			_ = stream.Emit("phase_finished", map[string]any{"phase": record.Phase, "status": record.Status})
		}
	}
	switch {
	case result.Counts.Failed > 0:
		result.Status = "failed"
		result.ExitCode = firstExit(result.Failed)
	case result.Counts.Blocked > 0:
		result.Status = "blocked"
		result.ExitCode = firstExit(result.Blocked)
	case result.Counts.Skipped > 0:
		result.Status = "partial"
	default:
		result.Status = "passed"
		result.FullRegression = true
	}
	if stream != nil {
		_ = stream.Emit("run_finished", map[string]any{"status": result.Status, "full_regression": result.FullRegression})
	}
	return result
}

func firstExit(phases []report.Phase) int {
	if len(phases) > 0 && phases[0].ExitCode != nil {
		return *phases[0].ExitCode
	}
	return 1
}
