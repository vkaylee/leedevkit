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
	Status         string
	ExitCode       int
	Diagnostic     string
	LifecycleError string
	Command        string
	Log            string
}

type Executor interface {
	Execute(Phase) PhaseResult
}

type ContextExecutor interface {
	ExecuteContext(context.Context, Phase) PhaseResult
}

type ExecutorFunc func(Phase) PhaseResult

type ContextExecutorFunc func(context.Context, Phase) PhaseResult

func (f ContextExecutorFunc) Execute(phase Phase) PhaseResult {
	return f(context.Background(), phase)
}

func (f ContextExecutorFunc) ExecuteContext(ctx context.Context, phase Phase) PhaseResult {
	return f(ctx, phase)
}

type GatePhaseExecutor struct {
	Definitions         []gates.Gate
	Run                 func(context.Context, process.Request) process.Result
	ComposeRun          func(context.Context, []string, []string) process.Result
	Fix                 bool
	Done                map[string]bool
	BaseRef             string
	StructuredDefaultGo bool
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
	result := gates.Executor{Run: run, Fix: e.Fix, Done: e.Done, BaseRef: e.BaseRef, StructuredDefaultGo: e.StructuredDefaultGo}.RunPhase(ctx, PhaseKey(phase.Name), e.Definitions)
	return PhaseResult{Status: result.Status, ExitCode: result.ExitCode, Diagnostic: result.Diagnostic}
}

// PhaseKey maps a scheduled phase display name to its canonical gate phase key.
func PhaseKey(displayName string) string {
	switch displayName {
	case "Unit Tests":
		return "unit"
	case "Integration Tests":
		return "integration"
	case "Linting":
		return "lint"
	case "Coverage":
		return "coverage"
	default:
		return displayName
	}
}

// PhaseDependencyPrefix maps a scheduled phase display name and service language to its [test.dependencies] key prefix.
func PhaseDependencyPrefix(displayName, language string) string {
	switch displayName {
	case "Unit Tests":
		return "unit-"
	case "Integration Tests":
		if language == "typescript" {
			return "e2e-"
		}
		return "int-"
	case "Linting":
		return "lint-"
	default:
		return ""
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
	Failures       []report.Failure
	LifecycleError string
	FailureLogs    []string
}

func (r RunResult) Summary(target, runID string) report.Summary {
	nextAction := ""
	switch r.Status {
	case "passed":
		nextAction = "No further action required."
	case "partial":
		if r.ExitCode != 0 {
			nextAction = "Configure gates or dependencies for skipped required phases, or pass explicit skip flags."
		} else {
			nextAction = "Run without focused or skip flags for full regression verification."
		}
	case "failed", "blocked":
		nextAction = "Inspect failed phase logs and rerun the reported command."
	}
	return report.Summary{
		SchemaVersion:  1,
		RunID:          runID,
		Status:         r.Status,
		FullRegression: r.FullRegression,
		ExitCode:       r.ExitCode,
		Target:         target,
		Counts:         r.Counts,
		Phases:         r.Phases,
		Failed:         r.Failed,
		Failures:       r.Failures,
		LifecycleError: r.LifecycleError,
		Blocked:        r.Blocked,
		Skipped:        r.Skipped,
		NextAction:     nextAction,
	}
}

func Run(flags Flags, executor Executor, streamWriter io.Writer, finalize ...func(*RunResult)) RunResult {
	return RunContext(context.Background(), flags, executor, streamWriter, finalize...)
}

func RunContext(ctx context.Context, flags Flags, executor Executor, streamWriter io.Writer, finalize ...func(*RunResult)) RunResult {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	phases, err := Schedule(flags)
	if err != nil {
		phase := report.Phase{Phase: "Validation", Target: flags.Target, Status: "blocked", Required: true, Diagnostic: err.Error()}
		return RunResult{Status: "blocked", FullRegression: false, ExitCode: 2, Counts: report.Counts{Blocked: 1}, Phases: []report.Phase{phase}, Blocked: []report.Phase{phase}, Failures: []report.Failure{}}
	}
	runID := fmt.Sprintf("go-%d", started.UnixNano())
	var stream *report.Stream
	if streamWriter != nil && flags.JSONStream {
		stream, _ = report.NewStream(streamWriter, runID)
		_ = stream.Emit("run_started", map[string]any{"target": flags.Target})
	}
	result := RunResult{Failures: []report.Failure{}}
	fullRegression := !flags.Coverage && !flags.LintOnly && !flags.UnitOnly && !flags.E2EOnly && !flags.SkipLint && !flags.SkipE2E && !flags.SkipBuild && flags.Pattern == "" && flags.BaseRef == ""
	var unexpectedSkipped []report.Phase
	for _, phase := range phases {
		phaseStarted := time.Now().UTC()
		record := report.Phase{Phase: phase.Name, Target: flags.Target, Status: phase.Status, Required: phase.Status == "selected", StartTime: phaseStarted.Format(time.RFC3339Nano)}
		if stream != nil {
			_ = stream.Emit("phase_started", map[string]any{"phase": record.Phase, "target": record.Target})
		}
		if phase.Status == "selected" {
			if executor == nil {
				record.Status = "blocked"
				record.Diagnostic = "phase executor is required"
			} else {
				var outcome PhaseResult
				if contextual, ok := executor.(ContextExecutor); ok {
					outcome = contextual.ExecuteContext(ctx, phase)
				} else {
					outcome = executor.Execute(phase)
				}
				record.Status = outcome.Status
				record.Command = outcome.Command
				record.Log = outcome.Log
				record.Diagnostic = outcome.Diagnostic
				record.LifecycleError = outcome.LifecycleError
				if outcome.Status == "failed" || outcome.Status == "blocked" {
					record.Failure = &report.Failure{Phase: record.Phase, Message: outcome.Diagnostic}
					result.Failures = append(result.Failures, *record.Failure)
				}
				if outcome.LifecycleError != "" {
					result.LifecycleError = outcome.LifecycleError
				}
				code := outcome.ExitCode
				record.ExitCode = &code
			}
		}
		phaseFinished := time.Now().UTC()
		record.EndTime = phaseFinished.Format(time.RFC3339Nano)
		record.DurationS = phaseFinished.Sub(phaseStarted).Seconds()
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
			if record.Required {
				unexpectedSkipped = append(unexpectedSkipped, record)
			}
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
	case len(unexpectedSkipped) > 0:
		result.Status = "partial"
		result.ExitCode = 1
		for _, skipped := range unexpectedSkipped {
			message := skipped.Diagnostic
			if message == "" {
				message = "required phase was unexpectedly skipped"
			}
			result.Failures = append(result.Failures, report.Failure{Phase: skipped.Phase, Message: message})
		}
	case result.Counts.Skipped > 0 || !fullRegression:
		result.Status = "partial"
	default:
		result.Status = "passed"
		result.FullRegression = true
	}
	for _, finish := range finalize {
		finish(&result)
	}
	if stream != nil {
		_ = stream.Emit("run_finished", map[string]any{"status": result.Status, "full_regression": result.FullRegression, "summary": result.Summary(flags.Target, runID)})
	}
	return result
}

func firstExit(phases []report.Phase) int {
	if len(phases) > 0 && phases[0].ExitCode != nil && *phases[0].ExitCode != 0 {
		return *phases[0].ExitCode
	}
	return 1
}
