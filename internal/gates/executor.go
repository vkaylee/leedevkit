package gates

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

type Executor struct {
	Run                 func(context.Context, process.Request) process.Result
	Fix                 bool
	Done                map[string]bool
	BaseRef             string
	StructuredDefaultGo bool
}

type PhaseResult struct {
	Status     string
	ExitCode   int
	Diagnostic string
	Gate       string
}

func (e Executor) RunPhase(ctx context.Context, phase string, definitions []Gate) (result PhaseResult) {
	run := e.Run
	if run == nil {
		run = process.Run
	}
	if e.Done == nil {
		e.Done = map[string]bool{}
	}
	for _, gate := range definitions {
		if gate.Phase != phase || !conditionMatches(gate.Condition) {
			continue
		}
		for _, required := range gate.Requires {
			if !e.Done[required] {
				return PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "required gate did not pass: " + required, Gate: gate.Name}
			}
		}
		if err := checkVersion(ctx, run, gate); err != nil {
			return PhaseResult{Status: "failed", ExitCode: err.ExitCode, Diagnostic: err.Diagnostic, Gate: gate.Name}
		}
		if err := runCommand(ctx, run, gate, gate.SetupCommand); err != nil {
			_ = runCommand(ctx, run, gate, gate.TeardownCommand)
			return PhaseResult{Status: "failed", ExitCode: err.ExitCode, Diagnostic: err.Diagnostic, Gate: gate.Name}
		}
		func() {
			defer func() {
				if teardown := runCommand(ctx, run, gate, gate.TeardownCommand); teardown != nil && result.Status == "passed" {
					result = PhaseResult{Status: "failed", ExitCode: teardown.ExitCode, Diagnostic: teardown.Diagnostic, Gate: gate.Name}
				}
			}()
			command := append([]string(nil), gate.Command...)
			if e.Fix && len(gate.FixCommand) > 0 {
				command = append([]string(nil), gate.FixCommand...)
			} else if gate.Mode == "check" && gate.Name == "gofmt" {
				command = []string{"gofmt", "-l", "."}
			}
			if e.StructuredDefaultGo && gate.Name == "go-test" && slicesEqual(command, []string{"go", "test", "./..."}) {
				command = []string{"go", "test", "-json", "./..."}
			}
			if e.BaseRef != "" && len(gate.NewFindingsFlag) > 0 {
				command = append(command, gate.NewFindingsFlag...)
				command = append(command, e.BaseRef)
			}
			if commandResult := runCommandWith(ctx, run, gate, command); commandResult != nil {
				result = PhaseResult{Status: "failed", ExitCode: commandResult.ExitCode, Diagnostic: commandResult.Diagnostic, Gate: gate.Name}
			} else {
				result = PhaseResult{Status: "passed", Gate: gate.Name}
			}
		}()
		if result.Status == "failed" {
			switch {
			case gate.FailurePolicy == "optional" && !gate.Required:
				result.Status = "passed"
			case gate.FailurePolicy == "block":
				result.Status = "blocked"
				return result
			default:
				return result
			}
		}
		e.Done[gate.Name] = true
	}
	return PhaseResult{Status: "passed"}
}

type commandResult struct {
	ExitCode   int
	Diagnostic string
}

func checkVersion(ctx context.Context, run func(context.Context, process.Request) process.Result, gate Gate) *commandResult {
	if gate.ExpectedVersion == nil && len(gate.VersionCommand) == 0 {
		return nil
	}
	if gate.ExpectedVersion == nil || len(gate.VersionCommand) == 0 {
		return &commandResult{ExitCode: 1, Diagnostic: "version_command and expected_version are required together"}
	}
	commandCtx := ctx
	cancel := func() {}
	if gate.Timeout != nil && *gate.Timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, timeoutDuration(gate.Timeout))
	}
	defer cancel()
	var stdout bytes.Buffer
	version := run(commandCtx, process.Request{Args: gate.VersionCommand, Dir: filepath.Clean(gate.WorkingDir), Env: gate.Environment, Stdout: &stdout, Stderr: &stdout, Timeout: timeoutDuration(gate.Timeout)})
	if version.ExitCode != 0 {
		return &commandResult{ExitCode: version.ExitCode, Diagnostic: errorText(version.Err)}
	}
	output := version.Stdout
	if len(output) == 0 {
		output = stdout.Bytes()
	}
	actual := strings.Fields(string(output))
	expected := strings.Fields(*gate.ExpectedVersion)
	if len(expected) == 0 || len(actual) < len(expected) || !slicesEqual(actual[:len(expected)], expected) {
		return &commandResult{ExitCode: 1, Diagnostic: fmt.Sprintf("expected %q, got %q", *gate.ExpectedVersion, strings.TrimSpace(string(output)))}
	}
	return nil
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func runCommand(ctx context.Context, run func(context.Context, process.Request) process.Result, gate Gate, command []string) *commandResult {
	if len(command) == 0 {
		return nil
	}
	return runCommandWith(ctx, run, gate, command)
}

func runCommandWith(ctx context.Context, run func(context.Context, process.Request) process.Result, gate Gate, command []string) *commandResult {
	commandCtx := ctx
	cancel := func() {}
	if gate.Timeout != nil && *gate.Timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, time.Duration(*gate.Timeout*float64(time.Second)))
	}
	defer cancel()
	result := run(commandCtx, process.Request{Args: command, Dir: filepath.Clean(gate.WorkingDir), Env: gate.Environment, Timeout: timeoutDuration(gate.Timeout)})
	if result.ExitCode != 0 {
		return &commandResult{ExitCode: result.ExitCode, Diagnostic: errorText(result.Err)}
	}
	return nil
}

func timeoutDuration(timeout *float64) time.Duration {
	if timeout == nil || *timeout <= 0 {
		return 0
	}
	return time.Duration(*timeout * float64(time.Second))
}

func conditionMatches(condition string) bool {
	return condition == "" || condition == "always"
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
