package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/provider"
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

type providerProtocolRequest struct {
	SchemaVersion int                       `json:"schema_version"`
	Provider      provider.ProtocolProvider `json:"provider"`
}

type providerProtocolResponse struct {
	SchemaVersion int            `json:"schema_version"`
	Diagnostics   map[string]any `json:"diagnostics"`
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

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--resolve-provider" {
		var request providerProtocolRequest
		if err := json.NewDecoder(stdin).Decode(&request); err != nil {
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
		if err := json.NewDecoder(stdin).Decode(&request); err != nil {
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
	case "test":
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
