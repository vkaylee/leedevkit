package gates

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

type Executor struct {
	Run                 func(context.Context, process.Request) process.Result
	RunGate             func(context.Context, Gate, process.Request) process.Result
	LookupEnv           func(name string) (present bool, forward bool)
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
		effectiveEnv, missing := e.resolveGateEnv(gate)
		if len(missing) > 0 {
			return PhaseResult{
				Status:     "blocked",
				ExitCode:   2,
				Diagnostic: fmt.Sprintf("missing required environment variable(s) for gate %q: %s", gate.Name, strings.Join(missing, ", ")),
				Gate:       gate.Name,
			}
		}
		gateRun := run
		if e.RunGate != nil {
			currentGate := gate
			gateRun = func(ctx context.Context, req process.Request) process.Result {
				return e.RunGate(ctx, currentGate, req)
			}
		}
		if err := checkVersion(ctx, gateRun, gate, effectiveEnv); err != nil {
			return PhaseResult{Status: "failed", ExitCode: err.ExitCode, Diagnostic: err.Diagnostic, Gate: gate.Name}
		}
		if err := runCommand(ctx, gateRun, gate, gate.SetupCommand, effectiveEnv); err != nil {
			_ = runCommand(ctx, gateRun, gate, gate.TeardownCommand, effectiveEnv)
			return PhaseResult{Status: "failed", ExitCode: err.ExitCode, Diagnostic: err.Diagnostic, Gate: gate.Name}
		}
		func() {
			defer func() {
				if teardown := runCommand(ctx, gateRun, gate, gate.TeardownCommand, effectiveEnv); teardown != nil && result.Status == "passed" {
					result = PhaseResult{Status: "failed", ExitCode: teardown.ExitCode, Diagnostic: teardown.Diagnostic, Gate: gate.Name}
				}
			}()
			command := append([]string(nil), gate.Command...)
			if e.Fix && len(gate.FixCommand) > 0 {
				command = append([]string(nil), gate.FixCommand...)
			} else if e.StructuredDefaultGo && len(gate.StructuredCommand) > 0 {
				command = append([]string(nil), gate.StructuredCommand...)
			}
			if e.BaseRef != "" && len(gate.NewFindingsFlag) > 0 {
				command = append(command, gate.NewFindingsFlag...)
				command = append(command, e.BaseRef)
			}
			if commandResult := runCommandWith(ctx, gateRun, gate, command, effectiveEnv); commandResult != nil {
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

func (e Executor) resolveGateEnv(gate Gate) ([]string, []string) {
	var missing []string
	var extraForward []string
	for _, name := range gate.RequiredEnv {
		if val, ok := gate.Env[name]; ok && val != "" {
			continue
		}
		if e.LookupEnv != nil {
			present, forward := e.LookupEnv(name)
			if !present {
				missing = append(missing, name)
			} else if forward {
				extraForward = append(extraForward, name)
			}
			continue
		}
		if val, ok := os.LookupEnv(name); ok && val != "" {
			extraForward = append(extraForward, name)
		} else {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, missing
	}
	if len(gate.Env) == 0 && len(extraForward) == 0 {
		return gate.Environment, nil
	}
	seen := map[string]bool{}
	env := make([]string, 0, len(gate.Environment)+len(extraForward)+len(gate.Env))
	for _, name := range gate.Environment {
		if _, hasStatic := gate.Env[name]; hasStatic || seen[name] {
			continue
		}
		seen[name] = true
		env = append(env, name)
	}
	for _, name := range extraForward {
		if _, hasStatic := gate.Env[name]; hasStatic || seen[name] {
			continue
		}
		seen[name] = true
		env = append(env, name)
	}
	if len(gate.Env) > 0 {
		keys := make([]string, 0, len(gate.Env))
		for key := range gate.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			env = append(env, key+"="+gate.Env[key])
		}
	}
	return env, nil
}

type commandResult struct {
	ExitCode   int
	Diagnostic string
}

func checkVersion(ctx context.Context, run func(context.Context, process.Request) process.Result, gate Gate, env []string) *commandResult {
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
	version := run(commandCtx, process.Request{Args: gate.VersionCommand, Dir: filepath.Clean(gate.WorkingDir), Env: env, Stdout: &stdout, Stderr: &stdout, Timeout: timeoutDuration(gate.Timeout)})
	if version.ExitCode != 0 || version.Err != nil {
		code := version.ExitCode
		if code == 0 {
			code = 1
		}
		return &commandResult{ExitCode: code, Diagnostic: errorText(version.Err)}
	}
	output := version.Stdout
	if len(output) == 0 {
		output = stdout.Bytes()
	}
	actual := strings.Fields(string(output))
	expected := strings.Fields(*gate.ExpectedVersion)
	if len(expected) == 0 || len(actual) < len(expected) || !slices.Equal(actual[:len(expected)], expected) {
		return &commandResult{ExitCode: 1, Diagnostic: fmt.Sprintf("expected %q, got %q", *gate.ExpectedVersion, strings.TrimSpace(string(output)))}
	}
	return nil
}

func runCommand(ctx context.Context, run func(context.Context, process.Request) process.Result, gate Gate, command []string, env []string) *commandResult {
	if len(command) == 0 {
		return nil
	}
	return runCommandWith(ctx, run, gate, command, env)
}

func runCommandWith(ctx context.Context, run func(context.Context, process.Request) process.Result, gate Gate, command []string, env []string) *commandResult {
	commandCtx := ctx
	cancel := func() {}
	if gate.Timeout != nil && *gate.Timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, time.Duration(*gate.Timeout*float64(time.Second)))
	}
	defer cancel()
	var checkOut bytes.Buffer
	request := process.Request{Args: command, Dir: filepath.Clean(gate.WorkingDir), Env: env, Timeout: timeoutDuration(gate.Timeout)}
	failOnOutput := gate.FailOnOutput || (gate.Mode == "check" && gate.Name == "gofmt")
	if failOnOutput {
		request.Stdout = &checkOut
	}
	result := run(commandCtx, request)
	if result.ExitCode != 0 || result.Err != nil {
		code := result.ExitCode
		if code == 0 {
			code = 1
		}
		return &commandResult{ExitCode: code, Diagnostic: errorText(result.Err)}
	}
	if failOnOutput {
		out := result.Stdout
		if len(out) == 0 {
			out = checkOut.Bytes()
		}
		if unformatted := strings.TrimSpace(string(out)); unformatted != "" {
			return &commandResult{ExitCode: 1, Diagnostic: "unformatted Go files: " + unformatted}
		}
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
