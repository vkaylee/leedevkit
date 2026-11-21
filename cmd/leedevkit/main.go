package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/modes"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
	runplan "github.com/vkaylee/leedevkit/internal/run"
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
  test     Run tests       (leedevkit test infra --lint-only)
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

func mustExecutable() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return executable
}

func runtimeRoot(executable string) string {
	project := findProjectRoot()
	return runtimeRootFor(executable, project, environMap(os.Environ()))
}

func runtimeRootFor(executable, project string, environ map[string]string) string {
	expected := projectVersion(project)
	candidates := make([]string, 0, 12)
	if explicit := environ["DEVKIT_HOME"]; explicit != "" {
		candidates = append(candidates, explicit)
	}
	if project != "" {
		candidates = append(candidates, filepath.Join(project, ".leedevkit"))
	}
	if home := environ["HOME"]; home != "" {
		candidates = append(candidates, filepath.Join(home, ".leedevkit", "current"))
	} else if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".leedevkit", "current"))
	}
	candidate := filepath.Dir(executable)
	for range 8 {
		candidates = append(candidates, candidate)
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
		candidate = parent
	}

	for _, root := range candidates {
		if runtimeCompatible(root, expected) {
			return root
		}
	}
	return ""
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'")
	return strings.TrimPrefix(value, "v")
}

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

func envProviderName(env map[string]string) string {
	if value := env["CONTAINER_PROVIDER"]; value != "" {
		return value
	}
	return "docker"
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

func runtimeCompatible(root, expected string) bool {
	if root == "" || !fileExists(filepath.Join(root, "scripts", "_orchestrator.py")) {
		return false
	}
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return false
	}
	actual := normalizeVersion(string(version))
	expected = normalizeVersion(expected)
	return expected == "" || expected == "latest" || actual == expected
}

func projectVersion(project string) string {
	if project == "" {
		return ""
	}
	content, err := os.ReadFile(filepath.Join(project, "leedevkit.toml"))
	if err != nil {
		return ""
	}
	section := false
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = line == "[devkit]"
			continue
		}
		if !section || !strings.HasPrefix(line, "version") {
			continue
		}
		_, value, found := strings.Cut(line, "=")
		if found {
			return normalizeVersion(value)
		}
	}
	return ""
}

func withRuntimeEnv(runtime string, values []string) []string {
	result := make([]string, 0, len(values)+1)
	for _, value := range values {
		if strings.HasPrefix(value, "DEVKIT_HOME=") {
			continue
		}
		result = append(result, value)
	}
	return append(result, "DEVKIT_HOME="+runtime)
}

func infraLintCommands(runtime, project string) [][]string {
	commands := [][]string{
		{filepath.Join(runtime, ".venv", "bin", "ruff"), "check", filepath.Join(runtime, "scripts")},
		{filepath.Join(runtime, ".venv", "bin", "mypy"), filepath.Join(runtime, "scripts")},
	}
	if _, err := exec.LookPath("shellcheck"); err == nil {
		shellFiles := []string{}
		if matches, globErr := filepath.Glob(filepath.Join(project, "*.sh")); globErr == nil {
			shellFiles = append(shellFiles, matches...)
		}
		if matches, globErr := filepath.Glob(filepath.Join(runtime, "scripts", "*.sh")); globErr == nil {
			shellFiles = append(shellFiles, matches...)
		}
		if len(shellFiles) > 0 {
			commands = append(commands, append([]string{"shellcheck"}, shellFiles...))
		}
	}
	return commands
}

func runInfraLint(runtime, project string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, args := range infraLintCommands(runtime, project) {
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
	}
	return 0
}

func validateInfraPattern(pattern string) (string, error) {
	if pattern == "" {
		return "", nil
	}
	for _, character := range pattern {
		if character < 32 || character == 127 {
			return "", fmt.Errorf("pattern cannot contain control characters")
		}
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return "", fmt.Errorf("invalid pattern regex: %w", err)
	}
	return pattern, nil
}

func infraUnitCommand(runtime, pattern string) []string {
	command := []string{
		filepath.Join(runtime, ".venv", "bin", "pytest"),
		"--cov=scripts",
		"--cov-config",
		filepath.Join(runtime, "scripts", ".coveragerc"),
		"--cov-report=term-missing",
		"--cov-report=html:.test_logs/coverage-production",
	}
	if pattern == "" {
		command = append(command, "--cov-fail-under=80")
	} else {
		command = append(command, "-p", "_pytest_pattern", "--leedevkit-pattern", pattern)
	}
	files, _ := filepath.Glob(filepath.Join(runtime, "scripts", "tests", "test_*.py"))
	return append(command, files...)
}

func runInfraUnit(runtime, project, pattern string, stdin io.Reader, stdout, stderr io.Writer) int {
	validated, err := validateInfraPattern(pattern)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	args := infraUnitCommand(runtime, validated)
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

func isInfraLint(args []string) bool {
	return len(args) >= 3 && args[0] == "test" && args[1] == "infra" && args[2] == "--lint-only"
}

func isInfraUnit(args []string) bool {
	return len(args) >= 3 && args[0] == "test" && args[1] == "infra" && args[2] == "--unit-only"
}

func runPythonCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Go core cannot dispatch an empty command")
		return 2
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "Go core cannot locate executable: %v\n", err)
		return 2
	}
	runtime := runtimeRoot(executable)
	if runtime == "" {
		fmt.Fprintln(stderr, "Python compatibility runtime missing")
		return 2
	}
	orchestrator := filepath.Join(runtime, "scripts", "_orchestrator.py")
	ensureVenv := filepath.Join(runtime, "scripts", "_ensure-venv.sh")
	if _, err := os.Stat(orchestrator); err != nil {
		fmt.Fprintf(stderr, "Python compatibility runtime missing: %s\n", orchestrator)
		return 2
	}
	if args[0] == "doctor" {
		python, err := exec.LookPath("python3")
		if err != nil {
			fmt.Fprintf(stderr, "Python interpreter missing: %v\n", err)
			return 2
		}
		command := exec.Command(python, append([]string{filepath.Join(runtime, "scripts", "_doctor.py")}, args[1:]...)...)
		command.Stdin = stdin
		command.Stdout = stdout
		command.Stderr = stderr
		command.Env = withRuntimeEnv(runtime, os.Environ())
		if err := command.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode()
			}
			return 2
		}
		return 0
	}
	if _, err := os.Stat(ensureVenv); err != nil {
		fmt.Fprintf(stderr, "Python runtime bootstrap missing: %s\n", ensureVenv)
		return 2
	}
	profile := "runtime"
	if args[0] == "test" {
		profile = "test"
	}
	venv := exec.Command(ensureVenv, profile)
	venv.Stdin = stdin
	venv.Stderr = stderr
	venv.Env = os.Environ()
	pythonPath, err := venv.Output()
	if err != nil {
		fmt.Fprintf(stderr, "Python runtime bootstrap failed: %v\n", err)
		return 2
	}
	python := strings.TrimSpace(string(pythonPath))
	if python == "" {
		fmt.Fprintln(stderr, "Python runtime bootstrap returned empty interpreter")
		return 2
	}
	command := exec.Command(python, append([]string{orchestrator}, args...)...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = withRuntimeEnv(runtime, os.Environ())
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "Python compatibility command failed: %v\n", err)
		return 2
	}
	return 0
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
		if len(args) < 2 {
			fmt.Fprintln(stderr, "test target is required")
			return 2
		}
		flags := testplan.Flags{Target: args[1]}
		projectConfig, configErr := config.Load(filepath.Join(projectRootOrCWD(), "leedevkit.toml"))
		if configErr != nil {
			fmt.Fprintf(stderr, "invalid project config: %v\n", configErr)
			return 2
		}
		flags.BuildPolicy = projectConfig.BuildPolicy
		configuredPlan := testplan.RequestFromConfig(args[1], projectConfig)
		if len(configuredPlan.Services) == 0 && args[1] != "all" {
			fmt.Fprintf(stderr, "test target %q has no configured services\n", args[1])
			return 2
		}
		for _, flag := range args[2:] {
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
			case "--json-stream":
				flags.JSONStream = true
			}
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
			compose := gates.ComposeRunner{Compose: composeTool, Project: project, Service: "go", Workdir: "/workspace", Execute: process.Run}
			if service, ok := projectConfig.ServiceForLanguage("go"); ok {
				workdir := "/workspace"
				if service.Workdir != "" {
					workdir = filepath.Join("/workspace", service.Workdir)
				}
				compose = compose.WithService(service.Name, workdir)
			}
			dependencies := projectConfig.Dependencies["int-go"]
			serviceConfig, _ := projectConfig.ServiceForLanguage("go")
			lifecyclePlan := lifecycle.Plan{Provider: providerConfig, Project: project, Profiles: []string{"--profile", "go"}, Services: []string{"go"}, Dependencies: dependencies, SkipBuild: flags.SkipBuild, OwnsProject: true}
			buildCallback, _, buildErr := buildCallbackForService(projectRootOrCWD(), project, lifecyclePlan, serviceConfig, projectConfig.BuildPolicy, flags.SkipBuild, newBuildRunner(projectRootOrCWD(), project, stderr))
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
			readiness := lifecycle.Readiness{Run: process.Run}
			readinessArgs := append(append([]string(nil), composeTool...), "-p", project, "ps", "--status", "running")
			lifecycleExecutor := lifecycle.Executor{
				UpArgs: upArgs, DownArgs: downArgs, Execute: func(ctx context.Context, args []string) error {
					return lifecycle.ComposeExecutor{Run: process.Run}.Execute(ctx, args)
				}, Readiness: func(ctx context.Context) error {
					return readiness.Wait(ctx, readinessArgs, []string{compose.Service}, 120*time.Second)
				},
			}
			result := testplan.Run(flags, testplan.ComposePhaseExecutor{
				Build: func(ctx context.Context, _ testplan.Phase) error {
					if buildCallback == nil {
						return nil
					}
					return buildCallback(ctx)
				},
				Lifecycle: lifecycleExecutor,
				Gate: func(ctx context.Context, phase testplan.Phase) testplan.PhaseResult {
					return testplan.GatePhaseExecutor{Definitions: projectConfig.Gates, ComposeRun: compose.Run, Fix: flags.Fix}.ExecuteContext(ctx, phase)
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

		if isInfraLint(args) || isInfraUnit(args) {
			project := findProjectRoot()
			runtime := runtimeRootFor(mustExecutable(), project, environMap(os.Environ()))
			if runtime == "" {
				fmt.Fprintln(stderr, "Python compatibility runtime missing")
				return 2
			}
			if isInfraLint(args) {
				return runInfraLint(runtime, project, stdin, stdout, stderr)
			}
			pattern := ""
			if len(args) >= 5 && args[3] == "--pattern" {
				pattern = args[4]
			}
			return runInfraUnit(runtime, project, pattern, stdin, stdout, stderr)
		}
		return runPythonCommand(args, stdin, stdout, stderr)
	case "dev", "debug", "init", "doctor", "skills", "update":
		return runPythonCommand(args, stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\nRun 'leedevkit' without arguments for usage.\n", args[0])
		return 1
	}
}
