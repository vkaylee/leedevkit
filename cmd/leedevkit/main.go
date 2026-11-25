package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/debug"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
	runplan "github.com/vkaylee/leedevkit/internal/run"
	runtimepkg "github.com/vkaylee/leedevkit/internal/runtime"
	"github.com/vkaylee/leedevkit/internal/skills"
)

const usage = `leedevkit — Enterprise DevKit for AI-first projects

Usage: leedevkit <command> [args...]

Commands:
  test     Run tests       (leedevkit test all)
  exec     Hermetic exec   (leedevkit exec <service> -- <cmd...>)
  targets  List configured targets, services, and phases
  dev      Dev environment (leedevkit dev up|down|restart|watch|migrate)
  mcp      MCP servers     (leedevkit mcp --stdio | leedevkit mcp playwright)
  init     Initialize a project with leedevkit
  update   Update the devkit to the latest (or pinned) release (alias: upgrade)
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

func configuredServiceName(project config.Project, language string) string {
	if service, ok := project.ServiceForLanguage(language); ok && service.Name != "" {
		return service.Name
	}
	return language
}

func runtimeAssetRoot() (string, error) {
	executable, err := runtimepkg.CurrentExecutablePath()
	if err != nil {
		return "", fmt.Errorf("resolve executable for global assets: %w", err)
	}
	return assetRootFromExecutable(executable)
}

func assetRootFromExecutable(executable string) (string, error) {
	root := runtimepkg.AssetRootFromExecutable(executable)
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

func composeBaseArgs(compose []string, project string, files []string) []string {
	args := append([]string{}, compose...)
	args = append(args, "-p", project)
	for _, file := range files {
		args = append(args, "-f", file)
	}
	return args
}

func composePSJSONArgs(compose []string, project string, files []string) []string {
	return append(composeBaseArgs(compose, project, files), "ps", "--format", "json")
}

func waitComposeServiceReady(ctx context.Context, runner func(context.Context, process.Request) process.Result, compose []string, project string, files []string, service, dir string, env []string, timeout time.Duration) error {
	psArgs := composePSJSONArgs(compose, project, files)
	return (lifecycle.Readiness{Run: runner, Dir: dir, Env: env}).Wait(ctx, psArgs, []string{service}, timeout)
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

func withProjectEnv(env []string, projectKey, root, project, namespace string) []string {
	keyPrefix := projectKey + "="
	result := make([]string, 0, len(env)+5)
	for _, entry := range env {
		if strings.HasPrefix(entry, "LEEDEVKIT_PROJECT_ROOT=") ||
			strings.HasPrefix(entry, "PROJECT_ROOT=") ||
			strings.HasPrefix(entry, keyPrefix) ||
			strings.HasPrefix(entry, "COMPOSE_PROJECT_NAME=") ||
			strings.HasPrefix(entry, "LEEDEVKIT_PROJECT_NAMESPACE=") {
			continue
		}
		result = append(result, entry)
	}
	result = append(result,
		"LEEDEVKIT_PROJECT_ROOT="+root,
		"PROJECT_ROOT="+root,
		keyPrefix+project,
		"COMPOSE_PROJECT_NAME="+project,
	)
	if namespace != "" {
		result = append(result, "LEEDEVKIT_PROJECT_NAMESPACE="+namespace)
	}
	return result
}

func withTestProjectEnv(env []string, root, project, namespace string) []string {
	return withProjectEnv(env, "LEEDEVKIT_TEST_PROJECT", root, project, namespace)
}

func withDevProjectEnv(env []string, root, project, namespace string) []string {
	return withProjectEnv(env, "LEEDEVKIT_DEV_PROJECT", root, project, namespace)
}

func hasCapability(values []string, wanted string) bool {
	return slices.Contains(values, wanted)
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

func runRunCLI(args []string, root string, stdout, stderr io.Writer) int {
	request, err := runplan.ParseCLI(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	plan, err := runplan.ResolveContext(request, environMap(os.Environ()), root)
	if err != nil {
		fmt.Fprintf(stderr, "invalid run request: %v\n", err)
		return 2
	}
	runCtx, stopRun := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRun()
	result := runplan.Execute(runCtx, runplan.ExecuteRequest{
		Plan: plan, Owned: plan.Owned,
		CleanupBase: runplan.CleanupBase(plan),
		LogRoot:     root, LogName: "run.log",
		Stdout: stdout, Stderr: stderr,
		RunCompose: func(argv []string) error {
			command := exec.Command(argv[0], argv[1:]...)
			command.Dir = root
			command.Stdout, command.Stderr = stderr, stderr
			return command.Run()
		},
	})
	if result.Err != nil {
		fmt.Fprintf(stderr, "run failed: %v\n", result.Err)
	}
	return result.Process.ExitCode
}

func runDebugCLI(args []string, defaultRoot string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, err := parseDebugMCPArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	root := options.ProjectRoot
	if root == "" {
		root = defaultRoot
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
	if err := debug.LaunchWithValidation(context.Background(), manifest, compose, engine, debug.ProcessComposeRunner(stdout, stderr), debug.ProcessComposeOutputRunner()); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if code, handled := dispatchProtocolFlag(args, stdin, stdout, stderr); handled {
		return code
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	root := projectRootOrCWD()
	switch args[0] {
	case "targets":
		projectConfig, err := config.Load(filepath.Join(root, "leedevkit.toml"))
		if err != nil {
			fmt.Fprintf(stderr, "invalid project config: %v\n", err)
			return 2
		}
		return printTargets(args, stdout, stderr, projectConfig)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return 0
	case "run":
		return runRunCLI(args, root, stdout, stderr)
	case "test":
		return runTestCLI(args, root, stdout, stderr)
	case "exec":
		return runExecCLI(args, root, stdin, stdout, stderr)
	case "doctor":
		if len(args) > 1 && args[1] == "--fix" {
			if err := repairDoctorRules(root); err != nil {
				fmt.Fprintf(stderr, "doctor rule repair failed: %v\n", err)
				return 2
			}
			if err := repairDoctorProjectArtifacts(root, stderr); err != nil {
				fmt.Fprintf(stderr, "doctor artifact repair failed: %v\n", err)
				return 2
			}
			sweepDoctorStaleProjects(root, stderr)
		}
		return runDoctor(root, stdout, stderr)
	case "dev":
		return runDevCLI(args, root, stdin, stdout, stderr)
	case "skills":
		if len(args) == 2 && args[1] == "list" {
			return runSkillsList(root, stdout, stderr)
		}
		action := append([]string(nil), args[1:]...)
		if err := (&skills.Manager{Root: root}).Run(action); err != nil {
			fmt.Fprintf(stderr, "skills failed: %v\n", err)
			return 2
		}
		return 0
	case "fmt:infra":
		fmt.Fprintln(stderr, "fmt:infra is not supported in Go-only runtime")
		return 2
	case "coverage":
		return runCoverageCLI(args, root, stdin, stdout, stderr)
	case "generated":
		return runGeneratedCLI(args, root, stdin, stdout, stderr)
	case "sync:api":
		return runAPISyncCLI(root, stdin, stdout, stderr)
	case "migrate:run", "migrate:revert", "migrate:status":
		return runMigrateCLI(args, root, stdout, stderr)
	case "db:query":
		return runDBQueryCLI(args, root, stdout, stderr)
	case "mcp":
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			fmt.Fprintln(stdout, "Usage: leedevkit mcp --stdio | leedevkit mcp playwright [flags]")
			return 0
		}
		if len(args) >= 2 && args[1] == "playwright" {
			return runPlaywrightMCP(args[2:], stdin, stdout, stderr, root)
		}
		if len(args) != 2 || args[1] != "--stdio" {
			fmt.Fprintln(stderr, "usage: leedevkit mcp --stdio | leedevkit mcp playwright [flags]")
			return 2
		}
		if err := serveProjectMCP(stdin, stdout, stderr, root); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	case "debug":
		return runDebugCLI(args, root, stdin, stdout, stderr)
	case "init":
		if err := runtimepkg.InitProject(root, os.Getenv("DEVKIT_LOCAL_PATH"), len(args) > 1 && args[1] == "--force"); err != nil {
			fmt.Fprintf(stderr, "init failed: %v\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "Project initialized.")
		return 0
	case "update", "upgrade":
		if len(args) > 2 {
			fmt.Fprintf(stderr, "usage: leedevkit %s [version]\n", args[0])
			return 2
		}
		target := ""
		if len(args) == 2 {
			target = args[1]
		}
		if err := runtimepkg.UpdateProject(root, target, os.Getenv("DEVKIT_LOCAL_PATH")); err != nil {
			fmt.Fprintf(stderr, "update failed: %v\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "Global DevKit updated.")
		warnLegacyProjectArtifacts(root, stderr)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\nRun 'leedevkit' without arguments for usage.\n", args[0])
		return 1
	}
}
