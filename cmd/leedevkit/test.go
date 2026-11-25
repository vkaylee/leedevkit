package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/ownership"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
	"github.com/vkaylee/leedevkit/internal/report"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

func openPhaseOutput(logDir, logName, prefix string, quiet bool, liveWriter io.Writer, appendMode bool) (*infraOutputWriter, string, error) {
	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(filepath.Join(logDir, logName), flags, 0o600)
	if err != nil {
		return nil, "", err
	}
	var live io.Writer = io.Discard
	if !quiet && liveWriter != nil {
		live = &infraPrefixWriter{writer: liveWriter, prefix: prefix}
	}
	relLog := filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))
	return &infraOutputWriter{log: file, live: live}, relLog, nil
}

func testDependencies(project config.Project, target string) []string {
	for _, key := range []string{"int-" + target, "int-go"} {
		if dependencies := project.Dependencies[key]; len(dependencies) > 0 {
			return append([]string(nil), dependencies...)
		}
	}
	return nil
}

func validateWebTestFlags(flags testplan.Flags) error {
	if flags.Pattern != "" || flags.Coverage || flags.Fix || flags.BaseRef != "" {
		return fmt.Errorf("web tests do not support --pattern, --coverage, --fix or --base-ref")
	}
	return nil
}

func webPhaseCommands(phase string) [][]string {
	switch phase {
	case "Unit Tests":
		return [][]string{{"bun", "run", "test"}}
	case "Integration Tests":
		return [][]string{{"bun", "run", "test:e2e"}}
	case "Linting":
		return [][]string{{"bun", "run", "lint"}, {"bun", "run", "type-check"}, {"bun", "run", "check-i18n"}}
	default:
		return nil
	}
}

func phaseDependencyKey(project config.Project, phase, target string) string {
	prefix := testplan.PhaseDependencyPrefix(phase, project.Services[target].Language)
	if prefix == "" {
		return ""
	}
	candidates := []string{prefix + target}
	aliases := make([]string, 0, len(project.Targets))
	for alias := range project.Targets {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		services := project.Targets[alias]
		if len(services) == 1 && services[0] == target {
			candidates = append(candidates, prefix+alias)
		}
	}
	for _, key := range candidates {
		if len(project.Dependencies[key]) > 0 {
			return key
		}
	}
	depKeys := make([]string, 0, len(project.Dependencies))
	for key := range project.Dependencies {
		if strings.HasPrefix(key, prefix) {
			depKeys = append(depKeys, key)
		}
	}
	sort.Strings(depKeys)
	for _, key := range depKeys {
		deps := project.Dependencies[key]
		if len(deps) > 0 && deps[len(deps)-1] == target {
			return key
		}
	}
	return candidates[0]
}

func phaseService(project config.Project, target string, phase string) ([]string, string, error) {
	phaseTarget := phaseDependencyKey(project, phase, target)
	dependencies := project.Dependencies[phaseTarget]
	if phaseTarget == "" || len(dependencies) == 0 {
		return nil, "", fmt.Errorf("test dependencies are not configured for %s", phaseTarget)
	}
	if _, ok := project.Services[target]; !ok {
		return nil, "", fmt.Errorf("test target %q is not a configured service", target)
	}
	service := dependencies[len(dependencies)-1]
	return append([]string(nil), dependencies...), service, nil
}

func configuredTargets(project config.Project, target string) ([]string, error) {
	selected := project.Targets[target]
	if len(selected) == 0 && target != "all" {
		if _, ok := project.Services[target]; ok {
			selected = []string{target}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("test target %q has no configured services", target)
	}
	services := make([]string, 0, len(selected))
	for _, selectedName := range selected {
		if _, ok := project.Services[selectedName]; ok {
			services = append(services, selectedName)
			continue
		}
		aliasServices, ok := project.Targets[selectedName]
		if !ok || len(aliasServices) == 0 {
			return nil, fmt.Errorf("test target %q references unknown service or alias %q", target, selectedName)
		}
		for _, service := range aliasServices {
			if _, ok := project.Services[service]; !ok {
				return nil, fmt.Errorf("test alias %q references unknown service %q", selectedName, service)
			}
			services = append(services, service)
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("test target %q has no configured services", target)
	}
	return services, nil
}

func buildWebLifecyclePlan(root, project string, providerConfig provider.Provider, configProject config.Project, target, phase string, flags testplan.Flags, skipBuild bool) (lifecycle.Plan, string, string, error) {
	service, ok := configProject.Services[target]
	if !ok || service.Language != "typescript" {
		return lifecycle.Plan{}, "", "", fmt.Errorf("web test target %q is not a configured TypeScript service", target)
	}
	phaseTarget := phaseDependencyKey(configProject, phase, target)
	dependencies, serviceName, err := phaseService(configProject, target, phase)
	if err != nil {
		return lifecycle.Plan{}, "", "", err
	}
	if len(dependencies) == 0 {
		return lifecycle.Plan{}, "", "", fmt.Errorf("web test dependencies are not configured for %s", phaseTarget)
	}
	if serviceName == "" {
		return lifecycle.Plan{}, "", "", fmt.Errorf("web service is not configured for %s", phaseTarget)
	}
	workdir := "/workspace"
	if service.Workdir != "" {
		workdir = filepath.Join(workdir, filepath.ToSlash(service.Workdir))
	} else if service.Dockerfile != "" {
		workdir = filepath.Join(workdir, strings.TrimPrefix(filepath.ToSlash(service.Dockerfile), "./"))
	} else {
		workdir = filepath.Join(workdir, target)
	}
	return lifecycle.Plan{Provider: providerConfig, Project: project, Files: []string{filepath.Join(root, ".compose", "docker-compose.test.yml")}, Profiles: []string{"--profile", phaseTarget}, Services: []string{serviceName}, Dependencies: append([]string(nil), dependencies[:len(dependencies)-1]...), Build: !skipBuild && flags.Build, SkipBuild: skipBuild, OwnsProject: true}, serviceName, workdir, nil
}

var projectNonceSeq atomic.Uint64

func nextProjectNonce() int64 {
	return time.Now().UnixNano() + int64(projectNonceSeq.Add(1))
}

func uniqueTestProject(root string) string {
	return fmt.Sprintf("leedevkit-test-%x-%x-%x", fnv32a(root), os.Getpid(), nextProjectNonce())
}

func sweepStaleTestProjects(ctx context.Context, providerConfig provider.Provider, namespace ...string) {
	if len(providerConfig.CLI) == 0 || !hasCapability(providerConfig.Capabilities, "ps") || !hasCapability(providerConfig.Capabilities, "rm") {
		return
	}
	ns := ""
	if len(namespace) > 0 {
		ns = namespace[0]
	}
	_ = ownership.SweepRuntime(ctx, ownership.RuntimeSweepOptions{CLI: providerConfig.CLI, Namespace: ns, MaxAge: time.Hour})
}

func fnv32a(value string) uint32 {
	var hash uint32 = 2166136261
	for index := 0; index < len(value); index++ {
		hash = (hash ^ uint32(value[index])) * 16777619
	}
	return hash
}

func resolveTestProjectName(env map[string]string, projectConfig config.Project, root string) string {
	if explicit := strings.TrimSpace(env["LEEDEVKIT_TEST_PROJECT"]); explicit != "" {
		return explicit
	}
	if projectConfig.Namespace != "" {
		return projectConfig.Namespace + "-test"
	}
	return uniqueTestProject(root)
}

func testProjectName(env map[string]string, root string) string {
	return resolveTestProjectName(env, config.Project{}, root)
}

func runWebPhase(root, project string, providerConfig provider.Provider, projectConfig config.Project, target, phase string, flags testplan.Flags, quiet bool, stdout, stderr io.Writer) testplan.PhaseResult {
	return runWebPhaseContext(context.Background(), root, project, providerConfig, projectConfig, target, phase, flags, quiet, stdout, stderr)
}

func runWebPhaseContext(ctx context.Context, root, project string, providerConfig provider.Provider, projectConfig config.Project, target, phase string, flags testplan.Flags, quiet bool, stdout, stderr io.Writer) testplan.PhaseResult {
	if err := ctx.Err(); err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "web phase canceled: " + process.RedactDiagnostic(err.Error())}
	}
	plan, service, workdir, err := buildWebLifecyclePlan(root, project, providerConfig, projectConfig, target, phase, flags, flags.SkipBuild)
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: err.Error()}
	}
	upArgs, err := plan.Up()
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: err.Error()}
	}
	downArgs, err := plan.Down()
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: err.Error()}
	}
	logDir, err := createInfraLogDir(root)
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable"}
	}
	logName := fmt.Sprintf("web-%s.log", strings.ToLower(strings.ReplaceAll(phase, " ", "-")))
	output, relLog, err := openPhaseOutput(logDir, logName, "[web] ", quiet || flags.JSONStream, stdout, false)
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable"}
	}
	testEnv := withTestProjectEnv(os.Environ(), root, project, projectConfig.Namespace)
	composeFile := filepath.Join(root, ".compose", "docker-compose.test.yml")
	readinessArgs := composePSJSONArgs(providerConfig.Compose, project, []string{composeFile})
	runner := lifecycle.Executor{UpArgs: upArgs, DownArgs: downArgs, ReadinessArgs: readinessArgs, Readiness: func(ctx context.Context) error {
		return waitComposeServiceReady(ctx, process.Run, providerConfig.Compose, project, []string{composeFile}, service, root, testEnv, 120*time.Second)
	}, Execute: func(ctx context.Context, args []string) error {
		return (lifecycle.ComposeExecutor{Run: process.Run, Dir: root, Env: testEnv, Stdout: output, Stderr: output}).Execute(ctx, args)
	}}
	var result testplan.PhaseResult
	err = runner.Run(ctx, func() error {
		compose := gates.ComposeRunner{Compose: providerConfig.Compose, Project: project, Service: service, Workdir: workdir, Dir: root, Env: testEnv, Execute: process.Run, Output: output}
		compose.Compose = append(append([]string(nil), compose.Compose...), "-f", composeFile)
		for _, command := range webPhaseCommands(phase) {
			if err := ctx.Err(); err != nil {
				result = testplan.PhaseResult{Status: "blocked", ExitCode: 1, Command: strings.Join(process.RedactArgs(command), " "), Log: relLog, Diagnostic: "web phase canceled: " + process.RedactDiagnostic(err.Error())}
				return err
			}
			processResult := compose.Run(ctx, command, nil)
			if processResult.ExitCode != 0 {
				result = testplan.PhaseResult{Status: "failed", ExitCode: processResult.ExitCode, Command: strings.Join(process.RedactArgs(command), " "), Log: relLog, Diagnostic: fmt.Sprintf("web phase command failed with exit code %d", processResult.ExitCode)}
				return fmt.Errorf("web phase failed")
			}
		}
		result = testplan.PhaseResult{Status: "passed"}
		return nil
	})
	if err != nil && result.Status == "" {
		result = testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: process.RedactDiagnostic(err.Error()), Log: relLog}
	} else if err != nil && result.Status == "passed" {
		status, exitCode := lifecycleFailureStatus(err)
		result.Status, result.ExitCode, result.Diagnostic = status, exitCode, process.RedactDiagnostic(err.Error())
		result.Log = relLog
	}
	if output.Close() != nil {
		return testplan.PhaseResult{Status: "failed", ExitCode: 1, Diagnostic: "test output write failed", Log: relLog}
	}
	var lifecycleErr *lifecycle.Failure
	if errors.As(err, &lifecycleErr) {
		result.LifecycleError = process.RedactDiagnostic(err.Error())
	}
	return result
}

func runConfiguredGatesAfterUp(ctx context.Context, execute func(context.Context, []string) error, up, down []string, run func(context.Context) error, readiness ...func(context.Context) error) error {
	executor := lifecycle.Executor{UpArgs: up, DownArgs: down, Execute: execute}
	if len(readiness) > 0 {
		executor.Readiness = readiness[0]
	}
	return executor.Run(ctx, func() error { return run(ctx) })
}

func lifecycleFailureStatus(err error) (string, int) {
	if err == nil {
		return "passed", 0
	}
	var failure *lifecycle.Failure
	if errors.As(err, &failure) && failure.Phase == "teardown" {
		return "failed", 1
	}
	return "blocked", 1
}

func phaseFailureStatus(gate gates.PhaseResult, lifecycleErr error) (string, int) {
	if gate.Status == "failed" || gate.Status == "blocked" {
		return gate.Status, gate.ExitCode
	}
	return lifecycleFailureStatus(lifecycleErr)
}

func goPhaseWithCompose(ctx context.Context, execute func(context.Context, []string) error, up, down []string, gatesExecutor func(context.Context) gates.PhaseResult, readiness ...func(context.Context) error) (gates.PhaseResult, error) {
	var outcome gates.PhaseResult
	err := runConfiguredGatesAfterUp(ctx, execute, up, down, func(ctx context.Context) error {
		outcome = gatesExecutor(ctx)
		if outcome.Status != "passed" {
			return fmt.Errorf("gate phase %s", outcome.Status)
		}
		return nil
	}, readiness...)
	return outcome, err
}

func runGoPhase(root, project string, providerConfig provider.Provider, configProject config.Project, target, phase string, flags testplan.Flags, quiet bool, stdout, stderr io.Writer) testplan.PhaseResult {
	return runGoPhaseContext(context.Background(), root, project, providerConfig, configProject, target, phase, flags, quiet, stdout, stderr)
}

func runGoPhaseContext(ctx context.Context, root, project string, providerConfig provider.Provider, configProject config.Project, target, phase string, flags testplan.Flags, quiet bool, stdout, stderr io.Writer) testplan.PhaseResult {
	if err := ctx.Err(); err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "Go phase canceled: " + process.RedactDiagnostic(err.Error())}
	}
	name := testplan.PhaseKey(phase)
	phaseGates := configProject.GatesForService(target, name)
	phaseKey := phaseDependencyKey(configProject, phase, target)
	dependencies, declared := configProject.Dependencies[phaseKey]
	if len(phaseGates) == 0 {
		if declared {
			return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: fmt.Sprintf("test.dependencies declares %q with no configured Go gates for %s", phaseKey, name)}
		}
		if phase == "Integration Tests" {
			return testplan.PhaseResult{Status: "skipped", Diagnostic: "no configured Go gates for integration"}
		}
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "no configured Go gates for " + name}
	}
	if len(dependencies) == 0 {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "Go test dependencies are not configured for " + phaseKey}
	}
	goService := dependencies[len(dependencies)-1]
	service, ok := configProject.Services[target]
	if !ok {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "Go test service is not configured: " + target}
	}
	workdir := "/workspace"
	if service.Workdir != "" {
		workdir = filepath.Join(workdir, filepath.ToSlash(service.Workdir))
	}
	plan := buildGoLifecyclePlanForPhase(root, project, providerConfig, configProject, target, phase, flags, flags.SkipBuild)
	up, err := plan.Up()
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: err.Error()}
	}
	down, err := plan.Down()
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: err.Error()}
	}
	logDir, err := createInfraLogDir(root)
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable"}
	}
	logName := "go-" + strings.ToLower(strings.ReplaceAll(phase, " ", "-")) + ".log"
	output, log, err := openPhaseOutput(logDir, logName, "[go] ", quiet || flags.JSONStream, stdout, false)
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable"}
	}
	testEnv := withTestProjectEnv(os.Environ(), root, project, configProject.Namespace)
	composeFile := filepath.Join(root, ".compose", "docker-compose.test.yml")
	composeArgs := composeBaseArgs(providerConfig.Compose, project, []string{composeFile})
	compose := gates.ComposeRunner{Compose: composeArgs, Project: project, Service: goService, Workdir: workdir, Dir: root, Env: testEnv, Execute: process.Run, Output: output}
	gateResult, lifecycleResult := goPhaseWithCompose(ctx, func(ctx context.Context, args []string) error {
		return (lifecycle.ComposeExecutor{Run: process.Run, Dir: root, Env: testEnv, Stdout: output, Stderr: output}).Execute(ctx, args)
	}, up, down, func(ctx context.Context) gates.PhaseResult {
		return gates.Executor{StructuredDefaultGo: configProject.DefaultGoGates && (quiet || flags.JSONStream), Fix: flags.Fix, BaseRef: flags.BaseRef, Run: func(ctx context.Context, request process.Request) process.Result {
			runner := compose
			if request.Dir != "" && request.Dir != "." {
				runner = runner.WithService("", filepath.ToSlash(filepath.Join("/workspace", request.Dir)))
			}
			runner.AllowTruncatedCapture = configProject.DefaultGoGates && (quiet || flags.JSONStream) && (name == "unit" || name == "integration") && len(request.Args) >= 4 && request.Args[0] == "go" && request.Args[1] == "test" && request.Args[2] == "-json"
			return runner.Run(ctx, request.Args, request.Env)
		}}.RunPhase(ctx, name, phaseGates)
	}, func(ctx context.Context) error {
		return waitComposeServiceReady(ctx, process.Run, providerConfig.Compose, project, []string{composeFile}, goService, root, testEnv, 120*time.Second)
	})
	closeErr := output.Close()
	if lifecycleResult != nil || gateResult.Status != "passed" || closeErr != nil {
		diagnostic := "Go phase failed"
		status := gateResult.Status
		exitCode := gateResult.ExitCode
		if lifecycleResult != nil {
			status, exitCode = phaseFailureStatus(gateResult, lifecycleResult)
		}
		if status == "" {
			status, exitCode = "failed", 1
		}
		if lifecycleResult != nil {
			diagnostic = process.RedactDiagnostic(lifecycleResult.Error())
		}
		if gateResult.Diagnostic != "" {
			diagnostic = gateResult.Diagnostic
		}
		if closeErr != nil {
			status, exitCode = "failed", 1
			diagnostic = "test output write failed"
		}
		result := testplan.PhaseResult{Status: status, ExitCode: exitCode, Diagnostic: diagnostic, Log: log}
		var lifecycleErr *lifecycle.Failure
		if errors.As(lifecycleResult, &lifecycleErr) {
			result.LifecycleError = process.RedactDiagnostic(lifecycleResult.Error())
		}
		return result
	}
	return testplan.PhaseResult{Status: "passed"}
}

func validateWebLifecycle(project config.Project, targets []string, flags testplan.Flags) error {
	selected, err := testplan.Schedule(flags)
	if err != nil {
		return err
	}
	for _, phase := range selected {
		if phase.Status != "selected" {
			continue
		}
		for _, target := range targets {
			if project.Services[target].Language != "typescript" {
				continue
			}
			if _, _, err := phaseService(project, target, phase.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func phaseStartupSucceeded(outcome testplan.PhaseResult) bool {
	return !strings.HasPrefix(outcome.LifecycleError, "startup:") && (outcome.Status == "passed" || outcome.Status == "failed")
}

func runMultiTargetTest(root string, flags testplan.Flags, projectConfig config.Project, stdout, stderr io.Writer, quiet bool) int {
	return runMultiTargetTestContext(context.Background(), root, flags, projectConfig, stdout, stderr, quiet)
}

func runMultiTargetTestContext(ctx context.Context, root string, flags testplan.Flags, projectConfig config.Project, stdout, stderr io.Writer, quiet bool) int {
	compact := flags.FailedOnly || flags.Compact
	quiet = quiet || compact
	flags.JSONStream = flags.JSONStream && !compact
	targets, err := configuredTargets(projectConfig, flags.Target)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var goTargets, webTargets []string
	for _, target := range targets {
		service := projectConfig.Services[target]
		switch service.Language {
		case "go":
			goTargets = append(goTargets, target)
		case "typescript":
			webTargets = append(webTargets, target)
		default:
			fmt.Fprintf(stderr, "test target %q uses unsupported language %q\n", target, service.Language)
			return 2
		}
	}
	if len(webTargets) == 0 && !hasGoPerPhaseComposeConfig(root, projectConfig, goTargets) {
		return -1
	}
	if len(goTargets) > 1 {
		fmt.Fprintln(stderr, "mixed test target supports at most one Go service")
		return 2
	}
	if len(webTargets) > 0 {
		if err := validateWebTestFlags(flags); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := validateWebLifecycle(projectConfig, webTargets, flags); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	env := environMap(os.Environ())
	providerConfig, err := resolveProviderForProject(root, env)
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	sweepStaleTestProjects(ctx, providerConfig, projectConfig.Namespace)
	project := resolveTestProjectName(env, projectConfig, root)
	ownedProject := !ownership.IsLocked(project)
	if session, err := ownership.Open(project, ownedProject); err == nil {
		defer session.Close(nil)
	}
	if env["COMPOSE_PROJECT_NAME"] != "" && env["COMPOSE_PROJECT_NAME"] != project {
		fmt.Fprintf(stderr, "COMPOSE_PROJECT_NAME is set; using test project %q\n", project)
	}
	built := map[string]bool{}
	result := testplan.RunContext(ctx, flags, testplan.ContextExecutorFunc(func(phaseCtx context.Context, phase testplan.Phase) testplan.PhaseResult {
		if phase.Status != "selected" {
			return testplan.PhaseResult{Status: "skipped"}
		}
		if err := phaseCtx.Err(); err != nil {
			return testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "test run canceled: " + process.RedactDiagnostic(err.Error())}
		}
		var partial *testplan.PhaseResult
		for _, target := range goTargets {
			if err := phaseCtx.Err(); err != nil {
				return testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "test run canceled: " + process.RedactDiagnostic(err.Error())}
			}
			phaseFlags := flags
			if phaseFlags.BuildPolicy == "once" && built[target] {
				phaseFlags.Build = false
			}
			outcome := runGoPhaseContext(phaseCtx, root, project, providerConfig, projectConfig, target, phase.Name, phaseFlags, quiet, stdout, stderr)
			if phaseFlags.BuildPolicy == "once" && phaseStartupSucceeded(outcome) {
				built[target] = true
			}
			if outcome.Status == "skipped" {
				copy := outcome
				partial = &copy
				continue
			}
			if outcome.Status != "passed" {
				return outcome
			}
		}
		for _, target := range webTargets {
			if err := phaseCtx.Err(); err != nil {
				return testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "test run canceled: " + process.RedactDiagnostic(err.Error())}
			}
			phaseFlags := flags
			if phaseFlags.BuildPolicy == "once" && built[target] {
				phaseFlags.Build = false
			}
			outcome := runWebPhaseContext(phaseCtx, root, project, providerConfig, projectConfig, target, phase.Name, phaseFlags, quiet, stdout, stderr)
			if phaseFlags.BuildPolicy == "once" && phaseStartupSucceeded(outcome) {
				built[target] = true
			}
			if outcome.Status != "passed" {
				return outcome
			}
		}
		if partial != nil {
			return *partial
		}
		return testplan.PhaseResult{Status: "passed"}
	}), func() io.Writer {
		if flags.JSONStream {
			return stdout
		}
		return nil
	}(), func(result *testplan.RunResult) {
		var failureOutput io.Writer
		if compact {
			failureOutput = stdout
		}
		collectFailureDetails(root, result, failureOutput)
	})
	if !flags.JSONStream {
		if err := json.NewEncoder(stdout).Encode(result.Summary(flags.Target, "go-cli")); err != nil {
			fmt.Fprintf(stderr, "failed to write test response: %v\n", err)
			return 2
		}
	}
	return result.ExitCode
}

func hasGoPerPhaseComposeConfig(root string, project config.Project, goTargets []string) bool {
	if len(goTargets) == 0 || !fileExists(filepath.Join(root, ".compose", "docker-compose.test.yml")) {
		return false
	}
	for _, target := range goTargets {
		for _, phase := range []string{"Unit Tests", "Integration Tests", "Linting"} {
			key := phaseDependencyKey(project, phase, target)
			if key != "" && key != "int-go" && len(project.Dependencies[key]) > 0 {
				return true
			}
		}
	}
	return false
}

func collectFailureDetails(root string, result *testplan.RunResult, output io.Writer) {
	result.Failures = []report.Failure{}
	seen := map[string]bool{}
	for _, phase := range append(append([]report.Phase(nil), result.Failed...), result.Blocked...) {
		paths := []string{phase.Log}
		if phase.Phase == "Infrastructure" && len(result.FailureLogs) > 0 {
			paths = result.FailureLogs
		}
		found := false
		if output != nil && phase.LifecycleError != "" {
			fmt.Fprintf(output, "[%s] %s\n", phase.Phase, report.StripANSI(phase.LifecycleError))
		}
		for _, path := range paths {
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			// ponytail: larger logs use the diagnostic fallback; stream parsing to lift this ceiling.
			const maxFailureLog = 16 << 20
			file, err := os.Open(filepath.Join(root, path))
			var data []byte
			if err == nil {
				data, err = io.ReadAll(io.LimitReader(file, maxFailureLog+1))
				if closeErr := file.Close(); err == nil {
					err = closeErr
				}
			}
			if err == nil && len(data) > maxFailureLog {
				result.Failures = append(result.Failures, report.Failure{Phase: phase.Phase, Message: "log exceeds parsing limit; inspect " + path + ": " + report.StripANSI(phase.Diagnostic)})
				found = true
				if output != nil {
					fmt.Fprintf(output, "[%s] Log exceeds parsing limit; inspect %s and rerun the reported command. %s\n", phase.Phase, path, report.StripANSI(phase.Diagnostic))
				}
				continue
			}
			if err != nil {
				if output != nil {
					fmt.Fprintf(output, "[%s] log unavailable: %s\n", phase.Phase, path)
				}
				continue
			}
			failures, filtered, reliable := report.ParseTestFailuresWithSupport(data, phase.Phase)
			if len(failures) > 0 {
				found = true
				result.Failures = append(result.Failures, failures...)
			}
			if output != nil {
				if reliable && len(failures) > 0 && strings.TrimSpace(filtered) != "" {
					fmt.Fprintf(output, "[%s] %s\n", phase.Phase, strings.TrimRight(filtered, "\n"))
				} else {
					// ponytail: unknown runners cannot be filtered reliably; add a structured adapter when supported.
					fmt.Fprintf(output, "[%s] Cannot filter this log accurately; inspect %s and rerun the reported command. %s\n", phase.Phase, path, report.StripANSI(phase.Diagnostic))
				}
			}
		}
		if !found {
			result.Failures = append(result.Failures, report.Failure{Phase: phase.Phase, Message: report.StripANSI(phase.Diagnostic)})
		}
	}
	for _, skipped := range result.Skipped {
		if skipped.Required {
			msg := report.StripANSI(skipped.Diagnostic)
			if msg == "" {
				msg = "required phase was unexpectedly skipped"
			}
			result.Failures = append(result.Failures, report.Failure{Phase: skipped.Phase, Message: msg})
			if output != nil {
				fmt.Fprintf(output, "[%s] %s\n", skipped.Phase, msg)
			}
		}
	}
}

func buildGoLifecyclePlanForPhase(root, project string, providerConfig provider.Provider, projectConfig config.Project, target, phase string, flags testplan.Flags, skipBuild bool) lifecycle.Plan {
	phaseKey := phaseDependencyKey(projectConfig, phase, target)
	dependencies := projectConfig.Dependencies[phaseKey]
	service := dependencies[len(dependencies)-1]
	return lifecycle.Plan{Provider: providerConfig, Project: project, Files: []string{filepath.Join(root, ".compose", "docker-compose.test.yml")}, Profiles: []string{"--profile", phaseKey}, Services: []string{service}, Dependencies: append([]string(nil), dependencies[:len(dependencies)-1]...), Build: !skipBuild && flags.Build, SkipBuild: skipBuild, NativeWait: hasCapability(providerConfig.Capabilities, "wait"), WaitTimeout: 120, OwnsProject: true}
}

func buildTestLifecyclePlan(root, project, target string, providerConfig provider.Provider, projectConfig config.Project, build, skipBuild bool) (lifecycle.Plan, string, error) {
	service := configuredServiceName(projectConfig, "go")
	files, err := testComposeFiles(root)
	if err != nil {
		return lifecycle.Plan{}, "", err
	}
	return lifecycle.Plan{
		Provider:     providerConfig,
		Project:      project,
		Files:        files,
		Profiles:     []string{"--profile", "go"},
		Services:     []string{service},
		Dependencies: testDependencies(projectConfig, target),
		Build:        !skipBuild && build,
		SkipBuild:    skipBuild,
		OwnsProject:  true,
	}, service, nil
}

func isInfraTarget(project config.Project, target string) bool {
	return target == "infra" || (target == "all" && len(project.Targets["all"]) == 1 && project.Targets["all"][0] == "infra")
}

func shouldUseGoTest(project config.Project, target string) bool {
	if target == "infra" {
		return true
	}
	if len(project.Services) == 0 {
		return false
	}
	selected := project.Targets[target]
	if target == "all" {
		selected = project.Targets["all"]
	}
	if len(selected) == 0 {
		for name := range project.Services {
			selected = append(selected, name)
		}
	}
	for _, name := range selected {
		service, ok := project.Services[name]
		if ok {
			if service.Language != "go" {
				return false
			}
			continue
		}
		if name == "go" {
			continue
		}
		return false
	}
	return true
}

func parseTestCLIArgs(args []string, root string, projectConfig config.Project) (testplan.Flags, bool, error) {
	flags := testplan.Flags{Target: args[1], BuildPolicy: projectConfig.BuildPolicy}
	quiet := false
	for index := 0; index < len(args[2:]); index++ {
		flag := args[index+2]
		switch flag {
		case "--build":
			flags.Build = true
		case "--lint-only":
			flags.LintOnly = true
		case "--unit-only":
			flags.UnitOnly = true
		case "--e2e-only":
			flags.E2EOnly = true
		case "--coverage":
			flags.Coverage = true
		case "--skip-lint":
			flags.SkipLint = true
		case "--skip-e2e":
			flags.SkipE2E = true
		case "--fix":
			flags.Fix = true
		case "--base-ref":
			if index+1 >= len(args[2:]) {
				return flags, false, errors.New("--base-ref requires a value")
			}
			resolved, err := resolveBaseRef(root, args[index+3])
			if err != nil {
				return flags, false, err
			}
			flags.BaseRef = resolved
			index++
		case "--pattern":
			if index+1 >= len(args[2:]) {
				return flags, false, errors.New("--pattern requires a value")
			}
			flags.Pattern = args[index+3]
			if flags.Pattern == "" {
				return flags, false, errors.New("--pattern requires a non-empty value")
			}
			index++
		case "--json-stream":
			flags.JSONStream = true
		case "--failed-only":
			flags.FailedOnly = true
			quiet = true
		case "--compact":
			flags.Compact = true
			quiet = true
		case "--json":
			quiet = true
			flags.JSON = true
		case "--quiet":
			quiet = true
		default:
			return flags, false, fmt.Errorf("unsupported test flag: %s", flag)
		}
	}
	if flags.FailedOnly || flags.Compact {
		flags.JSONStream = false
	}
	return flags, quiet, nil
}

func runTestCLI(args []string, root string, stdout, stderr io.Writer) int {
	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
		fmt.Fprintln(stdout, `Usage: leedevkit test <target> [flags]

Flags: --build --lint-only --unit-only --e2e-only --coverage --skip-lint --skip-e2e --fix --base-ref <ref> --pattern <regex> --json-stream --json --quiet --failed-only --compact`)
		return 0
	}
	if len(args) < 2 {
		fmt.Fprintln(stderr, "test target is required")
		return 2
	}
	projectConfig, configErr := config.Load(filepath.Join(root, "leedevkit.toml"))
	if configErr != nil {
		fmt.Fprintf(stderr, "invalid project config: %v\n", configErr)
		return 2
	}
	configuredPlan := testplan.RequestFromConfig(args[1], projectConfig)
	if len(configuredPlan.Services) == 0 && args[1] != "all" && !isInfraTarget(projectConfig, args[1]) {
		fmt.Fprintf(stderr, "test target %q has no configured services\n", args[1])
		return 2
	}
	flags, quiet, err := parseTestCLIArgs(args, root, projectConfig)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	testCtx, stopTest := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopTest()
	if !isInfraTarget(projectConfig, flags.Target) {
		if !shouldUseGoTest(projectConfig, flags.Target) || (flags.Pattern == "" && !flags.Coverage && hasGoPerPhaseComposeConfig(root, projectConfig, []string{flags.Target})) {
			code := runMultiTargetTestContext(testCtx, root, flags, projectConfig, stdout, stderr, quiet)
			if code >= 0 {
				return code
			}
		}
	}
	if isInfraTarget(projectConfig, flags.Target) {
		if err := validateInfraProject(root); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := validateInfraFlags(args[2:]); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		sweepStaleTestProjects(testCtx, provider.Provider{CLI: []string{"docker"}, Capabilities: []string{"ps", "rm"}})
		result := runInfraContext(testCtx, root, flags, process.Run, stdout, quiet)
		if !flags.JSONStream {
			summary := result.Summary(args[1], "go-cli")
			if err := json.NewEncoder(stdout).Encode(summary); err != nil {
				fmt.Fprintf(stderr, "failed to write test response: %v\n", err)
				return 2
			}
		}
		return result.ExitCode
	}
	return runSingleTargetGoTest(testCtx, root, flags, projectConfig, quiet, stdout, stderr)
}

func runSingleTargetGoTest(testCtx context.Context, root string, flags testplan.Flags, projectConfig config.Project, quiet bool, stdout, stderr io.Writer) int {
	goGates := projectConfig.Gates
	if flags.Pattern != "" {
		if flags.LintOnly || flags.E2EOnly || flags.Coverage || flags.Fix || flags.BaseRef != "" {
			fmt.Fprintln(stderr, "--pattern cannot be combined with lint-only, e2e-only, coverage, fix or base-ref")
			return 2
		}
		var err error
		goGates, err = patternGoGate(goGates, flags.Pattern)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		flags.UnitOnly = true
	}
	if !shouldUseGoTest(projectConfig, flags.Target) {
		fmt.Fprintf(stderr, "Go test path does not support target %q\n", flags.Target)
		return 2
	}
	env := environMap(os.Environ())
	project := strings.TrimSpace(env["LEEDEVKIT_TEST_PROJECT"])
	if project == "" {
		project = strings.TrimSpace(env["COMPOSE_PROJECT_NAME"])
	}
	ownedProject := false
	if project == "" {
		project = resolveTestProjectName(env, projectConfig, root)
		ownedProject = !ownership.IsLocked(project)
	}
	providerConfig, providerErr := resolveProviderForProject(root, env)
	if providerErr != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", providerErr)
		return 2
	}
	sweepStaleTestProjects(testCtx, providerConfig, projectConfig.Namespace)
	if ownedProject {
		if session, err := ownership.Open(project, true); err == nil {
			defer session.Close(nil)
		}
	}
	composeTool := providerConfig.Compose
	lifecyclePlan, goService, lifecyclePlanErr := buildTestLifecyclePlan(root, project, flags.Target, providerConfig, projectConfig, flags.Build, flags.SkipBuild)
	if lifecyclePlanErr != nil {
		fmt.Fprintf(stderr, "invalid lifecycle plan: %v\n", lifecyclePlanErr)
		return 2
	}
	testEnv := withTestProjectEnv(os.Environ(), root, project, projectConfig.Namespace)
	compose := gates.ComposeRunner{Compose: composeTool, Project: project, Service: goService, Workdir: "/workspace", Dir: root, Env: testEnv, Execute: process.Run}
	if service, ok := projectConfig.ServiceForLanguage("go"); ok {
		workdir := "/workspace"
		if service.Workdir != "" {
			workdir = filepath.Join("/workspace", service.Workdir)
		}
		compose = compose.WithService(service.Name, workdir)
	}
	logDir, logErr := createInfraLogDir(root)
	if logErr != nil {
		fmt.Fprintf(stderr, "test logging unavailable: %s\n", process.RedactDiagnostic(logErr.Error()))
		return 2
	}
	upArgs, upErr := lifecyclePlan.Up()
	nextPlan := lifecyclePlan
	nextPlan.Build = false
	nextUpArgs, nextUpErr := nextPlan.Up()
	downArgs, downErr := lifecyclePlan.Down()
	if upErr != nil || nextUpErr != nil || downErr != nil {
		fmt.Fprintf(stderr, "invalid lifecycle plan: %v %v %v\n", upErr, nextUpErr, downErr)
		return 2
	}
	readinessArgs := composePSJSONArgs(composeTool, project, lifecyclePlan.Files)
	built := false
	lifecycleExecutor := lifecycle.Executor{
		UpArgs: upArgs, DownArgs: downArgs, Execute: func(ctx context.Context, args []string) error {
			output, _, err := openPhaseOutput(logDir, "lifecycle.log", "[lifecycle] ", quiet || flags.JSONStream, stdout, true)
			if err != nil {
				return err
			}
			err = (lifecycle.ComposeExecutor{Run: process.Run, Dir: root, Env: testEnv, Stdout: output, Stderr: output}).Execute(ctx, args)
			if closeErr := output.Close(); closeErr != nil {
				return closeErr
			}
			if err == nil && slices.Equal(args, upArgs) {
				built = true
			}
			return err
		}, Readiness: func(ctx context.Context) error {
			return waitComposeServiceReady(ctx, process.Run, composeTool, project, lifecyclePlan.Files, compose.Service, root, testEnv, 120*time.Second)
		}, ReadinessArgs: readinessArgs, Report: func(phase string, args []string, err error) {
			if err != nil {
				fmt.Fprintf(stderr, "lifecycle %s failed: argv=%q error=%s\n", phase, process.RedactArgs(args), process.RedactDiagnostic(err.Error()))
			}
		},
	}
	gateIndex := 0
	compact := flags.FailedOnly || flags.Compact
	if compact {
		flags.JSONStream = false
		quiet = true
	}
	result := testplan.RunContext(testCtx, flags, testplan.ContextExecutorFunc(func(phaseCtx context.Context, phase testplan.Phase) testplan.PhaseResult {
		if err := phaseCtx.Err(); err != nil {
			return testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "test run canceled: " + process.RedactDiagnostic(err.Error())}
		}
		gatePhase := testplan.PhaseKey(phase.Name)
		var phaseGates []gates.Gate
		if flags.Pattern != "" {
			for _, g := range goGates {
				if g.Phase == gatePhase {
					phaseGates = append(phaseGates, g)
				}
			}
		} else {
			phaseGates = projectConfig.GatesForService(flags.Target, gatePhase)
		}
		if len(phaseGates) == 0 {
			if gatePhase == "integration" {
				if len(testDependencies(projectConfig, flags.Target)) > 0 {
					return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: fmt.Sprintf("test.dependencies declares integration dependencies for %q with no configured Go gates for integration", flags.Target)}
				}
				return testplan.PhaseResult{Status: "skipped", Diagnostic: "no configured Go gates for integration"}
			}
			return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "no configured Go gates for " + gatePhase}
		}
		phaseLifecycle := lifecycleExecutor
		if projectConfig.BuildPolicy == "once" && built {
			phaseLifecycle.UpArgs = nextUpArgs
		}
		return (testplan.ComposePhaseExecutor{
			Lifecycle: phaseLifecycle,
			Gate: func(ctx context.Context, phase testplan.Phase) testplan.PhaseResult {
				gateIndex++
				logName := fmt.Sprintf("gate-%02d.log", gateIndex)
				output, relativeLog, err := openPhaseOutput(logDir, logName, fmt.Sprintf("[gate-%02d] ", gateIndex), quiet || flags.JSONStream, stdout, false)
				if err != nil {
					return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable: " + process.RedactDiagnostic(err.Error())}
				}
				runner := compose
				runner.Output = output
				runGate := func(ctx context.Context, args []string, environment []string) process.Result {
					runner.AllowTruncatedCapture = projectConfig.DefaultGoGates && (quiet || flags.JSONStream) && flags.Pattern == "" && len(args) >= 4 && args[0] == "go" && args[1] == "test" && args[2] == "-json"
					result := runner.Run(ctx, args, environment)
					if flags.Pattern != "" && result.ExitCode == 0 {
						matched, err := patternGoEvents(bytes.NewReader(result.Stdout))
						if err != nil {
							result.ExitCode, result.Err = 1, err
						} else if !matched {
							result.ExitCode, result.Err = 1, fmt.Errorf("no tests matched --pattern")
						}
					}
					return result
				}
				outcome := (testplan.GatePhaseExecutor{Definitions: phaseGates, ComposeRun: runGate, Fix: flags.Fix, BaseRef: flags.BaseRef, StructuredDefaultGo: projectConfig.DefaultGoGates && (quiet || flags.JSONStream) && flags.Pattern == ""}).ExecuteContext(ctx, phase)
				if err := output.Close(); err != nil {
					return testplan.PhaseResult{Status: "failed", ExitCode: 1, Log: relativeLog, Diagnostic: "test output write failed: " + process.RedactDiagnostic(err.Error())}
				}
				if outcome.Status != "passed" {
					outcome.Log = relativeLog
				}
				return outcome
			},
		}).ExecuteContext(phaseCtx, phase)
	}), stdout, func(result *testplan.RunResult) {
		var failureOutput io.Writer
		if compact {
			failureOutput = stdout
		}
		collectFailureDetails(root, result, failureOutput)
	})
	if flags.JSONStream {
		return result.ExitCode
	}
	if err := json.NewEncoder(stdout).Encode(result.Summary(flags.Target, "go-cli")); err != nil {
		fmt.Fprintf(stderr, "failed to write test response: %v\n", err)
		return 2
	}
	return result.ExitCode
}
