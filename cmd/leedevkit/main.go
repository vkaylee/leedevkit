package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/dbcommands"
	"github.com/vkaylee/leedevkit/internal/debug"
	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/modes"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
	runplan "github.com/vkaylee/leedevkit/internal/run"
	runtimepkg "github.com/vkaylee/leedevkit/internal/runtime"
	"github.com/vkaylee/leedevkit/internal/skills"
	"github.com/vkaylee/leedevkit/internal/targets"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

type gateProtocolRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	Gates         []gates.Gate          `json:"gates"`
	Names         []string              `json:"names"`
	Definitions   map[string]gates.Gate `json:"definitions"`
}

type gateProtocolResponse struct {
	SchemaVersion int          `json:"schema_version"`
	Gates         []gates.Gate `json:"gates"`
}

type targetProtocolRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	Target        string                `json:"target"`
	AllServices   []string              `json:"all_services"`
	Aliases       []targetProtocolAlias `json:"aliases"`
}

type targetProtocolAlias struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

type targetProtocolResponse struct {
	SchemaVersion int      `json:"schema_version"`
	Targets       []string `json:"targets"`
}

type modeProtocolRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	Services      []modeProtocolService `json:"services"`
}

type modeProtocolService struct {
	Name     string `json:"name"`
	Language string `json:"language"`
}

type modeProtocolResponse struct {
	SchemaVersion int               `json:"schema_version"`
	Modes         map[string]string `json:"modes"`
}

type providerProtocolRequest struct {
	SchemaVersion int                       `json:"schema_version"`
	Provider      provider.ProtocolProvider `json:"provider"`
}

type providerProtocolResponse struct {
	SchemaVersion int            `json:"schema_version"`
	Diagnostics   map[string]any `json:"diagnostics"`
}

type lifecycleProtocolRequest struct {
	SchemaVersion int            `json:"schema_version"`
	Operation     string         `json:"operation"`
	Plan          lifecycle.Plan `json:"plan"`
}

type lifecycleProtocolResponse struct {
	SchemaVersion int      `json:"schema_version"`
	Operation     string   `json:"operation"`
	Args          []string `json:"args"`
}

type runProtocolRequest struct {
	SchemaVersion int             `json:"schema_version"`
	Plan          runplan.Request `json:"plan"`
}

type runProtocolResponse struct {
	SchemaVersion int      `json:"schema_version"`
	Args          []string `json:"args"`
}

type testProtocolResponse struct {
	SchemaVersion int                `json:"schema_version"`
	Result        testplan.RunResult `json:"result"`
}

const usage = `leedevkit — Enterprise DevKit for AI-first projects

Usage: leedevkit <command> [args...]

Commands:
  test     Run tests       (leedevkit test all)
  dev      Dev environment (leedevkit dev up|down)
  init     Initialize a project with leedevkit
  update   Update the devkit to the latest (or pinned) release
  doctor   System health check
  skills   Browse and install community skills
  version  Show devkit version

Run 'leedevkit <command> --help' for details.
`

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func goToolsRoot(project string) string { return project }

func environMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, item, found := strings.Cut(value, "=")
		if found {
			result[key] = item
		}
	}
	return result
}

func resolveProviderForProject(root string, env map[string]string) (provider.Provider, error) {
	repository := filepath.Join(root, "leedevkit.toml")
	return provider.Resolve(provider.Options{Repository: repository, Environment: env, CLI: env["LEEDEVKIT_CONTAINER_PROVIDER"]})
}

type debugMCPOptions struct {
	Environment string
	Manifest    string
	ProjectRoot string
	Compose     string
}

func parseDebugMCPArgs(args []string) (debugMCPOptions, error) {
	if len(args) < 2 || args[0] != "debug" || args[1] != "mcp" {
		return debugMCPOptions{}, fmt.Errorf("usage: debug mcp --environment NAME --manifest PATH")
	}
	options := debugMCPOptions{}
	for index := 2; index < len(args); index++ {
		if index+1 >= len(args) {
			return debugMCPOptions{}, fmt.Errorf("%s requires a value", args[index])
		}
		switch args[index] {
		case "--environment":
			options.Environment = args[index+1]
		case "--manifest":
			options.Manifest = args[index+1]
		case "--project-root":
			options.ProjectRoot = args[index+1]
		case "--compose-project":
			options.Compose = args[index+1]
		default:
			return debugMCPOptions{}, fmt.Errorf("unsupported debug argument %q", args[index])
		}
		index++
	}
	if options.Environment == "" || options.Manifest == "" {
		return debugMCPOptions{}, fmt.Errorf("debug mcp requires --environment and --manifest")
	}
	return options, nil
}

func envProviderName(env map[string]string) string {
	if value := env["CONTAINER_PROVIDER"]; value != "" {
		return value
	}
	return "docker"
}

func parseMigrationCommand(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("migration command accepts no arguments")
	}
	switch args[0] {
	case "migrate:run":
		return "run", nil
	case "migrate:revert":
		return "revert", nil
	case "migrate:status":
		return "status", nil
	default:
		return "", fmt.Errorf("unsupported migration command %q", args[0])
	}
}

func buildGoCoverageCommand(compose []string, service, pattern string, jsonOutput bool) ([]string, error) {
	if len(compose) == 0 || service == "" {
		return nil, fmt.Errorf("compose and coverage service are required")
	}
	goTest := "go test"
	if jsonOutput {
		goTest += " -json"
	}
	if pattern != "" {
		goTest += ` "$@"`
	}
	command := "mkdir -p /workspace/.test_logs && " + goTest + " -coverprofile=/workspace/.test_logs/coverage-go.out ./... && go tool cover -func=/workspace/.test_logs/coverage-go.out"
	args := append([]string{}, compose...)
	args = append(args, "exec", "-T", service, "-w", "/workspace", "sh", "-c", command)
	if pattern != "" {
		args = append(args, "coverage", "-run="+pattern)
	}
	return args, nil
}

func buildGeneratedGateCommand(compose []string, service, defaultService string, command []string, workdir string) ([]string, error) {
	if len(compose) == 0 || (service == "" && defaultService == "") || len(command) == 0 {
		return nil, fmt.Errorf("compose, service, and generated command are required")
	}
	if service == "" {
		service = defaultService
	}
	if workdir == "" {
		workdir = "/workspace"
	} else if !strings.HasPrefix(workdir, "/") {
		workdir = "/workspace/" + filepath.ToSlash(workdir)
	}
	args := append([]string{}, compose...)
	args = append(args, "exec", "-T", service, "-w", workdir)
	return append(args, command...), nil
}

func buildAPISyncCommands(compose []string, rustService, webService string) ([][]string, error) {
	if len(compose) == 0 || rustService == "" || webService == "" {
		return nil, fmt.Errorf("compose and API services are required")
	}
	first := append([]string{}, compose...)
	first = append(first, "exec", "-T", rustService, "bash", "-c", "cargo run -- gen-openapi > /workspace/shared/openapi.json")
	second := append([]string{}, compose...)
	second = append(second, "exec", "-T", webService, "bash", "-c", "npx openapi-typescript /workspace/shared/openapi.json -o ./src/types/api-generated.ts")
	return [][]string{first, second}, nil
}

func buildMigrationCommand(root string, config provider.Provider, project, service, action string) ([]string, error) {
	migrationArgs, err := dbcommands.MigrationArgs(action)
	if err != nil {
		return nil, err
	}
	return dbcommands.BuildDiesel(dbcommands.DieselRequest{
		Compose: config.Compose, Project: project, ComposeFile: filepath.Join(root, ".compose", "docker-compose.test.yml"),
		Root: root, WorkingDir: root, Service: service, Args: migrationArgs,
	})
}

func parseDatabaseQueryArgs(args []string) (string, bool, error) {
	if len(args) < 2 || args[0] != "db:query" || args[1] == "" || args[1] == "--json" {
		return "", false, fmt.Errorf("db:query requires SQL")
	}
	asJSON := false
	for _, arg := range args[2:] {
		if arg != "--json" {
			return "", false, fmt.Errorf("unsupported db:query argument %q", arg)
		}
		asJSON = true
	}
	return args[1], asJSON, nil
}

func buildDatabaseQueryArgs(config provider.Provider, project, sql string, asJSON bool) ([]string, error) {
	if len(config.CLI) == 0 || project == "" || sql == "" {
		return nil, fmt.Errorf("provider, project, and SQL are required")
	}
	args := append([]string{}, config.CLI...)
	args = append(args, "exec", "-t", project+"-db", "psql", "-U", "lee", "-d", "leedevkit", "-c", sql)
	if asJSON {
		args = append(args, "-A", "-t", "--json")
	}
	return args, nil
}

func configuredServiceName(project config.Project, language string) string {
	if service, ok := project.ServiceForLanguage(language); ok && service.Name != "" {
		return service.Name
	}
	return language
}

func runtimeAssetRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable for global assets: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path for global assets: %w", err)
	}
	return assetRootFromExecutable(executable)
}

func assetRootFromExecutable(executable string) (string, error) {
	binaryDir := filepath.Dir(executable)
	if filepath.Base(binaryDir) == "go" {
		binaryDir = filepath.Dir(binaryDir)
	}
	root := filepath.Dir(binaryDir)
	asset := filepath.Join(root, "container", "go", "docker-compose.test.yml")
	if !fileExists(asset) {
		return "", fmt.Errorf("global Compose asset not found beside executable: %s", asset)
	}
	return root, nil
}

func testComposeFiles(projectRoot string) ([]string, error) {
	custom := filepath.Join(projectRoot, ".compose", "docker-compose.test.yml")
	if fileExists(custom) {
		return []string{custom}, nil
	}
	assetRoot, err := runtimeAssetRoot()
	if err != nil {
		return nil, err
	}
	return []string{filepath.Join(assetRoot, "container", "go", "docker-compose.test.yml")}, nil
}

func withProjectRoot(env []string, root string) []string {
	result := make([]string, 0, len(env)+2)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "LEEDEVKIT_PROJECT_ROOT=") && !strings.HasPrefix(entry, "PROJECT_ROOT=") {
			result = append(result, entry)
		}
	}
	return append(result, "LEEDEVKIT_PROJECT_ROOT="+root, "PROJECT_ROOT="+root)
}

func hasCapability(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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
	prefix := map[string]string{"Unit Tests": "unit-", "Integration Tests": "int-", "Linting": "lint-"}[phase]
	if project.Services[target].Language == "typescript" && phase == "Integration Tests" {
		prefix = "e2e-"
	}
	if prefix == "" {
		return ""
	}
	candidates := []string{prefix + target}
	if project.Services[target].Language == "typescript" {
		aliases := make([]string, 0, len(project.Targets))
		for alias := range project.Targets {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		for _, alias := range aliases {
			for _, service := range project.Targets[alias] {
				if service == target {
					candidates = append([]string{prefix + alias}, candidates...)
				}
			}
		}
	}
	for _, key := range candidates {
		if len(project.Dependencies[key]) > 0 {
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
	return lifecycle.Plan{Provider: providerConfig, Project: project, Files: []string{filepath.Join(root, ".compose", "docker-compose.test.yml")}, Profiles: []string{"--profile", phaseTarget}, Services: []string{serviceName}, Dependencies: append([]string(nil), dependencies[:len(dependencies)-1]...), Build: !skipBuild && flags.Build, SkipBuild: skipBuild || !flags.Build, OwnsProject: true}, serviceName, workdir, nil
}

func uniqueTestProject(root string) string {
	return fmt.Sprintf("leedevkit-test-%x-%x-%x", fnv32a(root), os.Getpid(), time.Now().UnixNano())
}

func fnv32a(value string) uint32 {
	var hash uint32 = 2166136261
	for index := 0; index < len(value); index++ {
		hash = (hash ^ uint32(value[index])) * 16777619
	}
	return hash
}

func testProjectName(env map[string]string, root string) string {
	return uniqueTestProject(root)
}

func runWebPhase(root, project string, providerConfig provider.Provider, projectConfig config.Project, target, phase string, flags testplan.Flags, quiet bool, stdout, stderr io.Writer) testplan.PhaseResult {
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
	logPath := filepath.Join(logDir, logName)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable"}
	}
	defer file.Close()
	live := io.Writer(io.Discard)
	if !quiet && !flags.JSONStream {
		live = &infraPrefixWriter{writer: stdout, prefix: "[web] "}
	}
	output := &infraOutputWriter{log: file, live: live}
	readiness := lifecycle.Readiness{Run: process.Run, Dir: root, Env: withProjectRoot(os.Environ(), root)}
	readinessArgs := append(append([]string(nil), providerConfig.Compose...), "-p", project, "-f", filepath.Join(root, ".compose", "docker-compose.test.yml"), "ps", "--format", "json")
	runner := lifecycle.Executor{UpArgs: upArgs, DownArgs: downArgs, ReadinessArgs: readinessArgs, Readiness: func(ctx context.Context) error {
		return readiness.Wait(ctx, readinessArgs, []string{service}, 120*time.Second)
	}, Execute: func(ctx context.Context, args []string) error {
		result := process.Run(ctx, process.Request{Args: args, Dir: root, Env: withProjectRoot(os.Environ(), root), Stdout: output, Stderr: output})
		if result.ExitCode != 0 {
			return fmt.Errorf("compose exited with status %d", result.ExitCode)
		}
		return nil
	}}
	var result testplan.PhaseResult
	err = runner.Run(context.Background(), func() error {
		compose := gates.ComposeRunner{Compose: providerConfig.Compose, Project: project, Service: service, Workdir: workdir, Dir: root, Env: withProjectRoot(os.Environ(), root), Execute: process.Run, Output: output}
		compose.Compose = append(append([]string(nil), compose.Compose...), "-f", filepath.Join(root, ".compose", "docker-compose.test.yml"))
		for _, command := range webPhaseCommands(phase) {
			processResult := compose.Run(context.Background(), command, nil)
			if processResult.ExitCode != 0 {
				result = testplan.PhaseResult{Status: "failed", ExitCode: processResult.ExitCode, Command: strings.Join(process.RedactArgs(command), " "), Log: filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName)), Diagnostic: fmt.Sprintf("web phase command failed with exit code %d", processResult.ExitCode)}
				return fmt.Errorf("web phase failed")
			}
		}
		result = testplan.PhaseResult{Status: "passed"}
		return nil
	})
	if err != nil && result.Status == "" {
		result = testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: process.RedactDiagnostic(err.Error()), Log: filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))}
	} else if err != nil && result.Status == "passed" {
		status, exitCode := lifecycleFailureStatus(err)
		result.Status, result.ExitCode, result.Diagnostic = status, exitCode, process.RedactDiagnostic(err.Error())
		result.Log = filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))
	}
	if output.Close() != nil {
		return testplan.PhaseResult{Status: "failed", ExitCode: 1, Diagnostic: "test output write failed", Log: filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))}
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
	name := map[string]string{"Unit Tests": "unit", "Integration Tests": "integration", "Linting": "lint"}[phase]
	hasGate := false
	for _, gate := range configProject.Gates {
		if gate.Phase == name {
			hasGate = true
			break
		}
	}
	if !hasGate && phase == "Integration Tests" {
		return testplan.PhaseResult{Status: "skipped", Diagnostic: "no configured Go gates for integration"}
	}
	if !hasGate {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "no configured Go gates for " + name}
	}
	phaseKey := phaseDependencyKey(configProject, phase, target)
	dependencies := configProject.Dependencies[phaseKey]
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
	file, err := os.OpenFile(filepath.Join(logDir, logName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable"}
	}
	live := io.Writer(io.Discard)
	if !quiet && !flags.JSONStream {
		live = &infraPrefixWriter{writer: stdout, prefix: "[go] "}
	}
	output := &infraOutputWriter{log: file, live: live}
	composeArgs := append(append([]string(nil), providerConfig.Compose...), "-p", project, "-f", filepath.Join(root, ".compose", "docker-compose.test.yml"))
	compose := gates.ComposeRunner{Compose: composeArgs, Project: project, Service: goService, Workdir: workdir, Dir: root, Env: withProjectRoot(os.Environ(), root), Execute: process.Run, Output: output}
	gateResult, lifecycleResult := goPhaseWithCompose(context.Background(), func(ctx context.Context, args []string) error {
		result := process.Run(ctx, process.Request{Args: args, Dir: root, Env: withProjectRoot(os.Environ(), root), Stdout: output, Stderr: output})
		if result.ExitCode != 0 {
			return fmt.Errorf("compose exited with status %d", result.ExitCode)
		}
		return nil
	}, up, down, func(ctx context.Context) gates.PhaseResult {
		return gates.Executor{Run: func(ctx context.Context, request process.Request) process.Result {
			return compose.Run(ctx, request.Args, request.Env)
		}}.RunPhase(ctx, name, configProject.Gates)
	}, func(ctx context.Context) error {
		readiness := lifecycle.Readiness{Run: process.Run, Dir: root, Env: withProjectRoot(os.Environ(), root)}
		readinessArgs := append(append([]string(nil), providerConfig.Compose...), "-p", project, "-f", filepath.Join(root, ".compose", "docker-compose.test.yml"), "ps", "--format", "json")
		return readiness.Wait(ctx, readinessArgs, []string{goService}, 120*time.Second)
	})
	log := filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))
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
		return testplan.PhaseResult{Status: status, ExitCode: exitCode, Diagnostic: diagnostic, Log: log}
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

func runMultiTargetTest(root string, flags testplan.Flags, projectConfig config.Project, stdout, stderr io.Writer, quiet bool) int {
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
	if len(webTargets) == 0 {
		return -1
	}
	if len(goTargets) > 1 {
		fmt.Fprintln(stderr, "mixed test target supports at most one Go service")
		return 2
	}
	if err := validateWebTestFlags(flags); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := validateWebLifecycle(projectConfig, webTargets, flags); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	env := environMap(os.Environ())
	providerConfig, err := resolveProviderForProject(root, env)
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	project := testProjectName(env, root)
	if env["COMPOSE_PROJECT_NAME"] != "" {
		fmt.Fprintf(stderr, "COMPOSE_PROJECT_NAME is set; using isolated test project %q\n", project)
	}
	built := map[string]bool{}
	result := testplan.Run(flags, testplan.ExecutorFunc(func(phase testplan.Phase) testplan.PhaseResult {
		if phase.Status != "selected" {
			return testplan.PhaseResult{Status: "skipped"}
		}
		var partial *testplan.PhaseResult
		for _, target := range goTargets {
			phaseFlags := flags
			if phaseFlags.BuildPolicy == "once" && built[target] {
				phaseFlags.Build = false
			}
			outcome := runGoPhase(root, project, providerConfig, projectConfig, target, phase.Name, phaseFlags, quiet, stdout, stderr)
			if outcome.Status == "passed" && phaseFlags.BuildPolicy == "once" {
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
			phaseFlags := flags
			if phaseFlags.BuildPolicy == "once" && built[target] {
				phaseFlags.Build = false
			}
			outcome := runWebPhase(root, project, providerConfig, projectConfig, target, phase.Name, phaseFlags, quiet, stdout, stderr)
			if outcome.Status != "passed" {
				return outcome
			}
			if phaseFlags.BuildPolicy == "once" {
				built[target] = true
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
	}())
	if !flags.JSONStream {
		if err := json.NewEncoder(stdout).Encode(result.Summary(flags.Target, "go-cli")); err != nil {
			fmt.Fprintf(stderr, "failed to write test response: %v\n", err)
			return 2
		}
	}
	return result.ExitCode
}

func buildGoLifecyclePlanForPhase(root, project string, providerConfig provider.Provider, projectConfig config.Project, target, phase string, flags testplan.Flags, skipBuild bool) lifecycle.Plan {
	phaseKey := phaseDependencyKey(projectConfig, phase, target)
	dependencies := projectConfig.Dependencies[phaseKey]
	service := dependencies[len(dependencies)-1]
	return lifecycle.Plan{Provider: providerConfig, Project: project, Files: []string{filepath.Join(root, ".compose", "docker-compose.test.yml")}, Profiles: []string{"--profile", phaseKey}, Services: []string{service}, Dependencies: append([]string(nil), dependencies[:len(dependencies)-1]...), Build: !skipBuild && flags.Build, SkipBuild: skipBuild || !flags.Build, NativeWait: hasCapability(providerConfig.Capabilities, "wait"), WaitTimeout: 120, OwnsProject: true}
}

func buildTestLifecyclePlan(root, project, target string, providerConfig provider.Provider, projectConfig config.Project, skipBuild bool) (lifecycle.Plan, string, error) {
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

func envProviderCLI(env map[string]string) string {
	if value := env["CONTAINER_ENGINE"]; value != "" {
		return value
	}
	return "docker"
}

func splitCommand(value string) []string {
	return strings.Fields(value)
}

func projectRootOrCWD() string {
	if root := findProjectRoot(); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func findProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for current := cwd; ; current = filepath.Dir(current) {
		if fileExists(filepath.Join(current, "leedevkit.toml")) || fileExists(filepath.Join(current, ".git")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return cwd
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runCommand(runtime, project string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	command := exec.Command(args[0], args[1:]...)
	command.Dir = project
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 2
	}
	return 0
}

func resolveBaseRef(root, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	command := exec.Command("git", "rev-parse", "--verify", value+"^{commit}")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("base ref %q could not be resolved", value)
	}
	return strings.TrimSpace(string(output)), nil
}

func decodeSingleJSON(input io.Reader, target any) error {
	decoder := json.NewDecoder(input)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return fmt.Errorf("trailing JSON data: %w", err)
	}
	return nil
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--plan-lifecycle" {
		var request lifecycleProtocolRequest
		if err := decodeSingleJSON(stdin, &request); err != nil {
			fmt.Fprintf(stderr, "malformed lifecycle request: %v\n", err)
			return 2
		}
		if request.SchemaVersion != 1 {
			fmt.Fprintf(stderr, "unsupported lifecycle schema version: %d\n", request.SchemaVersion)
			return 2
		}
		operation := request.Operation
		if operation == "" {
			operation = "up"
		}
		var planned []string
		var err error
		switch operation {
		case "up":
			planned, err = request.Plan.Up()
		case "down":
			planned, err = request.Plan.Down()
		default:
			err = fmt.Errorf("unsupported lifecycle operation %q", operation)
		}
		if err != nil {
			fmt.Fprintf(stderr, "invalid lifecycle request: %v\n", err)
			return 2
		}
		if err := json.NewEncoder(stdout).Encode(lifecycleProtocolResponse{SchemaVersion: 1, Operation: operation, Args: planned}); err != nil {
			fmt.Fprintf(stderr, "failed to write lifecycle response: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) == 1 && args[0] == "--plan-run" {
		var request runProtocolRequest
		if err := decodeSingleJSON(stdin, &request); err != nil {
			fmt.Fprintf(stderr, "malformed run request: %v\n", err)
			return 2
		}
		if request.SchemaVersion != 1 {
			fmt.Fprintf(stderr, "unsupported run schema version: %d\n", request.SchemaVersion)
			return 2
		}
		planned, err := runplan.Plan(request.Plan)
		if err != nil {
			fmt.Fprintf(stderr, "invalid run request: %v\n", err)
			return 2
		}
		if err := json.NewEncoder(stdout).Encode(runProtocolResponse{SchemaVersion: 1, Args: planned}); err != nil {
			fmt.Fprintf(stderr, "failed to write run response: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) == 1 && args[0] == "--plan-test" {
		var request testplan.ProtocolRequest
		if err := decodeSingleJSON(stdin, &request); err != nil {
			fmt.Fprintf(stderr, "malformed test request: %v\n", err)
			return 2
		}
		if err := testplan.ValidateProtocol(request); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		result := testplan.Run(request.Flags, nil, nil)
		if err := json.NewEncoder(stdout).Encode(testProtocolResponse{SchemaVersion: 1, Result: result}); err != nil {
			fmt.Fprintf(stderr, "failed to write test response: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) == 1 && args[0] == "--resolve-modes" {
		var request modeProtocolRequest
		if err := decodeSingleJSON(stdin, &request); err != nil {
			fmt.Fprintf(stderr, "malformed mode request: %v\n", err)
			return 2
		}
		if request.SchemaVersion != 1 {
			fmt.Fprintf(stderr, "unsupported mode schema version: %d\n", request.SchemaVersion)
			return 2
		}
		services := make([]modes.Service, len(request.Services))
		for index, service := range request.Services {
			services[index] = modes.Service{Name: service.Name, Language: service.Language}
		}
		response := modeProtocolResponse{SchemaVersion: 1, Modes: modes.Resolve(services)}
		if err := json.NewEncoder(stdout).Encode(response); err != nil {
			fmt.Fprintf(stderr, "failed to write mode response: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) == 1 && args[0] == "--resolve-test-targets" {
		var request targetProtocolRequest
		if err := decodeSingleJSON(stdin, &request); err != nil {
			fmt.Fprintf(stderr, "malformed test target request: %v\n", err)
			return 2
		}
		if request.SchemaVersion != 1 {
			fmt.Fprintf(stderr, "unsupported test target schema version: %d\n", request.SchemaVersion)
			return 2
		}
		aliases := make([]targets.Alias, len(request.Aliases))
		for index, alias := range request.Aliases {
			aliases[index] = targets.Alias{Name: alias.Name, Services: alias.Services}
		}
		response := targetProtocolResponse{
			SchemaVersion: 1,
			Targets:       targets.Resolve(targets.Request{Target: request.Target, AllServices: request.AllServices, Aliases: aliases}),
		}
		if err := json.NewEncoder(stdout).Encode(response); err != nil {
			fmt.Fprintf(stderr, "failed to write test target response: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) == 1 && args[0] == "--resolve-provider" {
		var request providerProtocolRequest
		if err := decodeSingleJSON(stdin, &request); err != nil {
			fmt.Fprintf(stderr, "malformed provider request: %v\n", err)
			return 2
		}
		if request.SchemaVersion != 1 {
			fmt.Fprintf(stderr, "unsupported provider schema version: %d\n", request.SchemaVersion)
			return 2
		}
		diagnostics, err := provider.ValidateProtocol(request.Provider)
		if err != nil {
			fmt.Fprintf(stderr, "invalid provider request: %v\n", err)
			return 2
		}
		if err := json.NewEncoder(stdout).Encode(providerProtocolResponse{
			SchemaVersion: 1,
			Diagnostics:   diagnostics,
		}); err != nil {
			fmt.Fprintf(stderr, "failed to write provider response: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) == 1 && args[0] == "--plan-gates" {
		var request gateProtocolRequest
		if err := decodeSingleJSON(stdin, &request); err != nil {
			fmt.Fprintf(stderr, "malformed gate request: %v\n", err)
			return 2
		}
		if request.SchemaVersion != 1 {
			fmt.Fprintf(stderr, "unsupported gate schema version: %d\n", request.SchemaVersion)
			return 2
		}
		names := request.Names
		definitions := request.Definitions
		if len(names) == 0 && len(request.Gates) > 0 {
			definitions = make(map[string]gates.Gate, len(request.Gates))
			for _, gate := range request.Gates {
				definitions[gate.Name] = gate
				names = append(names, gate.Name)
			}
		}
		selected, err := gates.Select(names, definitions)
		if err != nil {
			fmt.Fprintf(stderr, "invalid gate request: %v\n", err)
			return 2
		}
		response := gateProtocolResponse{SchemaVersion: 1, Gates: selected}
		if err := json.NewEncoder(stdout).Encode(response); err != nil {
			fmt.Fprintf(stderr, "failed to write gate response: %v\n", err)
			return 2
		}
		return 0
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return 0
	case "run":
		request, err := runplan.ParseCLI(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		plan, err := runplan.ResolveContext(request, environMap(os.Environ()), projectRootOrCWD())
		if err != nil {
			fmt.Fprintf(stderr, "invalid run request: %v\n", err)
			return 2
		}
		result := runplan.Execute(context.Background(), runplan.ExecuteRequest{
			Plan: plan, Owned: plan.Owned,
			CleanupBase: append([]string(nil), plan.Compose...),
			LogRoot:     projectRootOrCWD(), LogName: "run.log",
			Stdout: stdout, Stderr: stderr,
			RunCompose: func(argv []string) error {
				command := exec.Command(argv[0], argv[1:]...)
				command.Dir = projectRootOrCWD()
				command.Stdout, command.Stderr = stderr, stderr
				return command.Run()
			},
		})
		if result.Err != nil {
			fmt.Fprintf(stderr, "run failed: %v\n", result.Err)
		}
		return result.Process.ExitCode
	case "test":
		if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
			fmt.Fprintln(stdout, `Usage: leedevkit test <target> [flags]

Flags: --lint-only --unit-only --e2e-only --coverage --skip-lint --skip-e2e --fix --base-ref <ref> --pattern <regex> --json-stream --json --quiet`)
			return 0
		}
		if len(args) < 2 {
			fmt.Fprintln(stderr, "test target is required")
			return 2
		}
		flags := testplan.Flags{Target: args[1]}
		quiet := false
		projectConfig, configErr := config.Load(filepath.Join(projectRootOrCWD(), "leedevkit.toml"))
		if configErr != nil {
			fmt.Fprintf(stderr, "invalid project config: %v\n", configErr)
			return 2
		}
		flags.BuildPolicy = projectConfig.BuildPolicy
		flags.Build = true
		for index, flag := range args[2:] {
			if flag == "--base-ref" {
				if index+3 >= len(args) {
					fmt.Fprintln(stderr, "--base-ref requires a value")
					return 2
				}
				resolved, err := resolveBaseRef(projectRootOrCWD(), args[index+3])
				if err != nil {
					fmt.Fprintln(stderr, err)
					return 2
				}
				flags.BaseRef = resolved
			}
		}
		configuredPlan := testplan.RequestFromConfig(args[1], projectConfig)
		if len(configuredPlan.Services) == 0 && args[1] != "all" && !isInfraTarget(projectConfig, args[1]) {
			fmt.Fprintf(stderr, "test target %q has no configured services\n", args[1])
			return 2
		}
		for index := 0; index < len(args[2:]); index++ {
			flag := args[index+2]
			switch flag {
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
					fmt.Fprintln(stderr, "--base-ref requires a value")
					return 2
				}
				index++
			case "--pattern":
				if index+1 >= len(args[2:]) {
					fmt.Fprintln(stderr, "--pattern requires a value")
					return 2
				}
				flags.Pattern = args[index+3]
				if flags.Pattern == "" {
					fmt.Fprintln(stderr, "--pattern requires a non-empty value")
					return 2
				}
				index++
			case "--json-stream":
				flags.JSONStream = true
			case "--json":
				quiet = true
				flags.JSON = true
			case "--quiet":
				quiet = true
			default:
				fmt.Fprintf(stderr, "unsupported test flag: %s\n", flag)
				return 2
			}
		}
		if !isInfraTarget(projectConfig, flags.Target) && !shouldUseGoTest(projectConfig, flags.Target) {
			code := runMultiTargetTest(projectRootOrCWD(), flags, projectConfig, stdout, stderr, quiet)
			if code >= 0 {
				return code
			}
		}
		if isInfraTarget(projectConfig, flags.Target) {
			if err := validateInfraProject(projectRootOrCWD()); err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			if err := validateInfraFlags(args[2:]); err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			result := runInfra(projectRootOrCWD(), flags, process.Run, stdout, quiet)
			if !flags.JSONStream {
				summary := result.Summary(args[1], "go-cli")
				if err := json.NewEncoder(stdout).Encode(summary); err != nil {
					fmt.Fprintf(stderr, "failed to write test response: %v\n", err)
					return 2
				}
			}
			return result.ExitCode
		}
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
		if flags.Target != "infra" {
			env := environMap(os.Environ())
			composeTool := splitCommand(env["DOCKER_COMPOSE_TOOL"])
			if len(composeTool) == 0 {
				composeTool = []string{"docker", "compose"}
			}
			project := env["COMPOSE_PROJECT_NAME"]
			if project == "" {
				project = "leedevkit-test"
			}
			providerConfig, providerErr := resolveProviderForProject(projectRootOrCWD(), env)
			if providerErr != nil {
				fmt.Fprintf(stderr, "invalid provider: %v\n", providerErr)
				return 2
			}
			composeTool = providerConfig.Compose
			lifecyclePlan, goService, lifecyclePlanErr := buildTestLifecyclePlan(projectRootOrCWD(), project, flags.Target, providerConfig, projectConfig, flags.SkipBuild)
			if lifecyclePlanErr != nil {
				fmt.Fprintf(stderr, "invalid lifecycle plan: %v\n", lifecyclePlanErr)
				return 2
			}
			compose := gates.ComposeRunner{Compose: composeTool, Project: project, Service: goService, Workdir: "/workspace", Dir: projectRootOrCWD(), Env: withProjectRoot(os.Environ(), projectRootOrCWD()), Execute: process.Run}
			if service, ok := projectConfig.ServiceForLanguage("go"); ok {
				workdir := "/workspace"
				if service.Workdir != "" {
					workdir = filepath.Join("/workspace", service.Workdir)
				}
				compose = compose.WithService(service.Name, workdir)
			}
			logDir, logErr := createInfraLogDir(projectRootOrCWD())
			if logErr != nil {
				fmt.Fprintf(stderr, "test logging unavailable: %s\n", process.RedactDiagnostic(logErr.Error()))
				return 2
			}
			buildLog, logErr := os.OpenFile(filepath.Join(logDir, "build.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if logErr != nil {
				fmt.Fprintf(stderr, "build logging unavailable: %s\n", process.RedactDiagnostic(logErr.Error()))
				return 2
			}
			defer buildLog.Close()
			var buildLive io.Writer = io.Discard
			if !quiet && !flags.JSONStream {
				buildLive = &infraPrefixWriter{writer: stderr, prefix: "[build] "}
			}
			buildOutput := &infraOutputWriter{log: buildLog, live: buildLive}
			serviceConfig, _ := projectConfig.ServiceForLanguage("go")
			buildCallback, _, buildErr := buildCallbackForService(projectRootOrCWD(), project, lifecyclePlan, serviceConfig, projectConfig.BuildPolicy, flags.SkipBuild, newBuildRunner(projectRootOrCWD(), project, buildOutput))
			if buildErr != nil {
				fmt.Fprintf(stderr, "invalid build plan: %v\n", buildErr)
				return 2
			}
			if buildCallback != nil {
				lifecyclePlan.Build = projectConfig.BuildPolicy == "phase"
			}
			upArgs, upErr := lifecyclePlan.Up()
			downArgs, downErr := lifecyclePlan.Down()
			if upErr != nil || downErr != nil {
				fmt.Fprintf(stderr, "invalid lifecycle plan: %v %v\n", upErr, downErr)
				return 2
			}
			readiness := lifecycle.Readiness{Run: process.Run, Dir: projectRootOrCWD(), Env: withProjectRoot(os.Environ(), projectRootOrCWD())}
			readinessArgs := append(append([]string(nil), composeTool...), "-p", project)
			for _, file := range lifecyclePlan.Files {
				readinessArgs = append(readinessArgs, "-f", file)
			}
			readinessArgs = append(readinessArgs, "ps", "--format", "json")
			lifecycleExecutor := lifecycle.Executor{
				UpArgs: upArgs, DownArgs: downArgs, Execute: func(ctx context.Context, args []string) error {
					file, err := os.OpenFile(filepath.Join(logDir, "lifecycle.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
					if err != nil {
						return err
					}
					var live io.Writer = io.Discard
					if !quiet && !flags.JSONStream {
						live = &infraPrefixWriter{writer: stdout, prefix: "[lifecycle] "}
					}
					output := &infraOutputWriter{log: file, live: live}
					err = (lifecycle.ComposeExecutor{Run: process.Run, Dir: projectRootOrCWD(), Env: withProjectRoot(os.Environ(), projectRootOrCWD()), Stdout: output, Stderr: output}).Execute(ctx, args)
					if closeErr := output.Close(); closeErr != nil {
						return closeErr
					}
					return err
				}, Readiness: func(ctx context.Context) error {
					return readiness.Wait(ctx, readinessArgs, []string{compose.Service}, 120*time.Second)
				}, ReadinessArgs: readinessArgs, Report: func(phase string, args []string, err error) {
					if err != nil {
						fmt.Fprintf(stderr, "lifecycle %s failed: argv=%q error=%s\n", phase, process.RedactArgs(args), process.RedactDiagnostic(err.Error()))
					}
				},
			}
			gateIndex := 0
			result := testplan.Run(flags, testplan.ComposePhaseExecutor{
				Build: func(ctx context.Context, _ testplan.Phase) error {
					if buildCallback == nil {
						return nil
					}
					err := buildCallback(ctx)
					if buildOutput.err != nil {
						return buildOutput.err
					}
					return err
				},
				Lifecycle: lifecycleExecutor,
				Gate: func(ctx context.Context, phase testplan.Phase) testplan.PhaseResult {
					gateIndex++
					logName := fmt.Sprintf("gate-%02d.log", gateIndex)
					logPath := filepath.Join(logDir, logName)
					relativeLog := filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))
					file, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
					if err != nil {
						return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "test logging unavailable: " + process.RedactDiagnostic(err.Error())}
					}
					var live io.Writer = io.Discard
					if !quiet && !flags.JSONStream {
						live = &infraPrefixWriter{writer: stdout, prefix: fmt.Sprintf("[gate-%02d] ", gateIndex)}
					}
					output := &infraOutputWriter{log: file, live: live}
					runner := compose
					runner.Output = output
					runGate := func(ctx context.Context, args []string, environment []string) process.Result {
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
					outcome := (testplan.GatePhaseExecutor{Definitions: goGates, ComposeRun: runGate, Fix: flags.Fix, BaseRef: flags.BaseRef}).ExecuteContext(ctx, phase)
					if err := output.Close(); err != nil {
						return testplan.PhaseResult{Status: "failed", ExitCode: 1, Log: relativeLog, Diagnostic: "test output write failed: " + process.RedactDiagnostic(err.Error())}
					}
					if outcome.Status != "passed" {
						outcome.Log = relativeLog
					}
					return outcome
				},
			}, stdout)
			if flags.JSONStream {
				return result.ExitCode
			}
			if err := json.NewEncoder(stdout).Encode(result.Summary(flags.Target, "go-cli")); err != nil {
				fmt.Fprintf(stderr, "failed to write test response: %v\n", err)
				return 2
			}
			return result.ExitCode
		}

		fmt.Fprintf(stderr, "unsupported Go test command: %s\n", strings.Join(args, " "))
		return 2
	case "doctor":
		if len(args) > 1 && args[1] == "--fix" {
			root := projectRootOrCWD()
			if err := repairDoctorRules(root); err != nil {
				fmt.Fprintf(stderr, "doctor rule repair failed: %v\n", err)
				return 2
			}
		}
		return runDoctor(projectRootOrCWD(), stdout, stderr)
	case "dev":
		if len(args) < 2 || (args[1] != "up" && args[1] != "down" && args[1] != "ps" && args[1] != "clean" && args[1] != "logs" && args[1] != "exec") {
			fmt.Fprintln(stderr, "unsupported dev command")
			return 2
		}
		providerConfig, err := resolveProviderForProject(projectRootOrCWD(), environMap(os.Environ()))
		if err != nil {
			fmt.Fprintf(stderr, "invalid provider: %v\n", err)
			return 2
		}
		service := ""
		var command []string
		if args[1] == "logs" && len(args) > 2 {
			service = args[2]
		}
		if args[1] == "exec" {
			if len(args) < 3 {
				fmt.Fprintln(stderr, "dev exec requires service")
				return 2
			}
			service = args[2]
			command = args[3:]
		}
		if args[1] == "up" || args[1] == "down" {
			return runDev(args[1], providerConfig, projectRootOrCWD(), stdout, stderr, process.Run)
		}
		argv, err := devComposeArgsWithOptions(providerConfig.Compose, args[1], service, command)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		result := process.Run(context.Background(), process.Request{Args: argv, Dir: projectRootOrCWD(), Stdout: stdout, Stderr: stderr})
		return result.ExitCode
	case "skills":
		if len(args) == 2 && args[1] == "list" {
			return runSkillsList(projectRootOrCWD(), stdout, stderr)
		}
		action := append([]string(nil), args[1:]...)
		if err := (&skills.Manager{Root: projectRootOrCWD()}).Run(action); err != nil {
			fmt.Fprintf(stderr, "skills failed: %v\n", err)
			return 2
		}
		return 0
	case "fmt:infra":
		fmt.Fprintln(stderr, "fmt:infra is not supported in Go-only runtime")
		return 2
	case "coverage":
		providerConfig, err := resolveProviderForProject(projectRootOrCWD(), environMap(os.Environ()))
		if err != nil {
			fmt.Fprintf(stderr, "invalid provider: %v\n", err)
			return 2
		}
		projectConfig, err := config.Load(filepath.Join(projectRootOrCWD(), "leedevkit.toml"))
		if err != nil {
			fmt.Fprintf(stderr, "invalid project config: %v\n", err)
			return 2
		}
		service := configuredServiceName(projectConfig, "go")
		pattern := ""
		jsonOutput := false
		for index := 1; index < len(args); index++ {
			switch args[index] {
			case "--pattern":
				if index+1 >= len(args) {
					fmt.Fprintln(stderr, "--pattern requires a value")
					return 2
				}
				pattern = args[index+1]
				index++
			case "--json":
				jsonOutput = true
			default:
				fmt.Fprintf(stderr, "unsupported coverage argument %q\n", args[index])
				return 2
			}
		}
		coverageArgs, err := buildGoCoverageCommand(providerConfig.Compose, service, pattern, jsonOutput)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return runCommand(projectRootOrCWD(), projectRootOrCWD(), coverageArgs, stdin, stdout, stderr)
	case "generated":
		providerConfig, err := resolveProviderForProject(projectRootOrCWD(), environMap(os.Environ()))
		if err != nil {
			fmt.Fprintf(stderr, "invalid provider: %v\n", err)
			return 2
		}
		projectConfig, err := config.Load(filepath.Join(projectRootOrCWD(), "leedevkit.toml"))
		if err != nil {
			fmt.Fprintf(stderr, "invalid project config: %v\n", err)
			return 2
		}
		service := configuredServiceName(projectConfig, "go")
		for _, gate := range projectConfig.Gates {
			if gate.Phase != "generated" {
				continue
			}
			command := gate.Command
			if len(args) > 1 && args[1] == "--fix" && len(gate.FixCommand) > 0 {
				command = gate.FixCommand
			}
			argv, err := buildGeneratedGateCommand(providerConfig.Compose, gate.Service, service, command, gate.WorkingDir)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			if code := runCommand(projectRootOrCWD(), projectRootOrCWD(), argv, stdin, stdout, stderr); code != 0 {
				return code
			}
		}
		return 0
	case "sync:api":
		providerConfig, err := resolveProviderForProject(projectRootOrCWD(), environMap(os.Environ()))
		if err != nil {
			fmt.Fprintf(stderr, "invalid provider: %v\n", err)
			return 2
		}
		projectConfig, err := config.Load(filepath.Join(projectRootOrCWD(), "leedevkit.toml"))
		if err != nil {
			fmt.Fprintf(stderr, "invalid project config: %v\n", err)
			return 2
		}
		rustService := configuredServiceName(projectConfig, "rust")
		webService := configuredServiceName(projectConfig, "web")
		commands, err := buildAPISyncCommands(providerConfig.Compose, rustService, webService)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		for _, command := range commands {
			if code := runCommand(projectRootOrCWD(), projectRootOrCWD(), command, stdin, stdout, stderr); code != 0 {
				return code
			}
		}
		return 0
	case "migrate:run", "migrate:revert", "migrate:status":
		action, err := parseMigrationCommand(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		providerConfig, err := resolveProviderForProject(projectRootOrCWD(), environMap(os.Environ()))
		if err != nil {
			fmt.Fprintf(stderr, "invalid provider: %v\n", err)
			return 2
		}
		env := environMap(os.Environ())
		project := env["COMPOSE_PROJECT_NAME"]
		if project == "" {
			project = "leedevkit-test"
		}
		service := "apiserver"
		if projectConfig, loadErr := config.Load(filepath.Join(projectRootOrCWD(), "leedevkit.toml")); loadErr == nil {
			service = configuredServiceName(projectConfig, "rust")
		}
		migrationArgs, err := buildMigrationCommand(projectRootOrCWD(), providerConfig, project, service, action)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		result := process.Run(context.Background(), process.Request{Args: migrationArgs, Dir: projectRootOrCWD(), Stdout: stdout, Stderr: stderr})
		return result.ExitCode
	case "db:query":
		sql, asJSON, err := parseDatabaseQueryArgs(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		providerConfig, err := resolveProviderForProject(projectRootOrCWD(), environMap(os.Environ()))
		if err != nil {
			fmt.Fprintf(stderr, "invalid provider: %v\n", err)
			return 2
		}
		env := environMap(os.Environ())
		project := env["COMPOSE_PROJECT_NAME"]
		if project == "" {
			project = "leedevkit-dev"
		}
		queryArgs, err := buildDatabaseQueryArgs(providerConfig, project, sql, asJSON)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		result := process.Run(context.Background(), process.Request{Args: queryArgs, Dir: projectRootOrCWD(), Stdout: stdout, Stderr: stderr})
		return result.ExitCode
	case "debug":
		options, err := parseDebugMCPArgs(args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		root := options.ProjectRoot
		if root == "" {
			root = projectRootOrCWD()
		}
		manifest, err := debug.Load(options.Manifest, options.Environment, debug.Options{ProjectRoot: root})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if len(os.Getenv("LEEDEVKIT_DEBUG_DIRECT")) > 0 {
			server := debug.NewServer(manifest)
			if err := server.Serve(stdin, stdout); err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			return 0
		}
		compose := splitCommand(os.Getenv("DOCKER_COMPOSE_TOOL"))
		if len(compose) == 0 {
			compose = []string{"docker", "compose"}
		}
		engine := []string{compose[0]}
		if len(compose) > 1 && compose[1] == "compose" {
			engine = []string{compose[0]}
		}
		if err := debug.LaunchWithValidation(context.Background(), manifest, compose, engine, debug.ProcessComposeRunner(stdout, stderr), debug.ProcessComposeOutputRunner()); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	case "init":
		root := projectRootOrCWD()
		if err := runtimepkg.InitProject(root, os.Getenv("DEVKIT_LOCAL_PATH"), len(args) > 1 && args[1] == "--force"); err != nil {
			fmt.Fprintf(stderr, "init failed: %v\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "Project initialized.")
		return 0
	case "update":
		if len(args) > 2 {
			fmt.Fprintln(stderr, "usage: leedevkit update [version]")
			return 2
		}
		target := ""
		if len(args) == 2 {
			target = args[1]
		}
		if err := runtimepkg.UpdateProject(projectRootOrCWD(), target, os.Getenv("DEVKIT_LOCAL_PATH")); err != nil {
			fmt.Fprintf(stderr, "update failed: %v\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "Global DevKit updated.")
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\nRun 'leedevkit' without arguments for usage.\n", args[0])
		return 1
	}
}
