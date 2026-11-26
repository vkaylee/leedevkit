package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/ownership"
	"github.com/vkaylee/leedevkit/internal/process"
)

const execUsage = `Usage: leedevkit exec [--dev] [-w|--workdir <dir>] <service> [--] <cmd...>

Run a non-test command hermetically inside a managed service container.
Ensures the container is running and executes in the service's configured working directory.
`

type execOptions struct {
	Dev     bool
	Workdir string
	Service string
	Command []string
	Help    bool
}

func parseExecArgs(args []string) (execOptions, error) {
	if len(args) == 0 || args[0] != "exec" {
		return execOptions{}, fmt.Errorf("usage: leedevkit exec [--dev] [-w|--workdir <dir>] <service> [--] <cmd...>")
	}
	var opts execOptions
	index := 1
	for index < len(args) {
		arg := args[index]
		switch arg {
		case "--help", "-h":
			opts.Help = true
			return opts, nil
		case "--dev":
			opts.Dev = true
			index++
		case "-w", "--workdir":
			if index+1 >= len(args) {
				return execOptions{}, fmt.Errorf("%s requires a value", arg)
			}
			opts.Workdir = args[index+1]
			index += 2
		default:
			if strings.HasPrefix(arg, "-") {
				return execOptions{}, fmt.Errorf("unsupported exec flag %q", arg)
			}
			opts.Service = arg
			rest := args[index+1:]
			if len(rest) > 0 && rest[0] == "--" {
				rest = rest[1:]
			}
			if len(rest) == 0 {
				return execOptions{}, fmt.Errorf("exec command is required after service %q", opts.Service)
			}
			for _, part := range rest {
				if part == "" {
					return execOptions{}, fmt.Errorf("exec command arguments must be non-empty")
				}
			}
			opts.Command = append([]string(nil), rest...)
			return opts, nil
		}
	}
	return execOptions{}, fmt.Errorf("exec requires <service> -- <cmd...>")
}

type resolvedExecTarget struct {
	ComposeService string
	Workdir        string
	Profiles       []string
	Dependencies   []string
	Files          []string
}

func resolveServiceWorkdir(root, serviceName string, service config.Service, override string) (string, error) {
	if override != "" {
		if strings.HasPrefix(override, "/workspace") {
			cleaned := filepath.ToSlash(filepath.Clean(override))
			if cleaned != "/workspace" && !strings.HasPrefix(cleaned, "/workspace/") {
				return "", fmt.Errorf("workdir %q must stay inside /workspace", override)
			}
			return cleaned, nil
		}
		if !config.IsRelativeWithinRoot(override) {
			return "", fmt.Errorf("workdir %q must be relative to project root", override)
		}
		cleaned := filepath.Clean(override)
		if cleaned == "." {
			return "/workspace", nil
		}
		return "/workspace/" + filepath.ToSlash(cleaned), nil
	}
	if service.Workdir != "" {
		return "/workspace/" + strings.TrimPrefix(filepath.ToSlash(filepath.Clean(service.Workdir)), "./"), nil
	}
	if service.Dockerfile != "" {
		candidate := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(service.Dockerfile)), "./")
		if filepath.Base(candidate) == "Dockerfile" {
			candidate = filepath.ToSlash(filepath.Dir(candidate))
		}
		if candidate != "" && candidate != "." {
			if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil && info.IsDir() {
				return "/workspace/" + candidate, nil
			}
			if service.Language == "typescript" || service.Language == "javascript" {
				return "/workspace/" + candidate, nil
			}
		}
	}
	if serviceName != "" {
		if info, err := os.Stat(filepath.Join(root, serviceName)); err == nil && info.IsDir() {
			return "/workspace/" + filepath.ToSlash(serviceName), nil
		}
	}
	return "/workspace", nil
}

func resolveExecTarget(root string, projectConfig config.Project, requested string, devMode bool, workdirOverride string) (resolvedExecTarget, error) {
	configKey := requested
	service, foundService := projectConfig.Services[configKey]
	if !foundService {
		if targets := projectConfig.Targets[requested]; len(targets) == 1 {
			if targetSvc, ok := projectConfig.Services[targets[0]]; ok {
				configKey = targets[0]
				service = targetSvc
				foundService = true
			}
		}
	}
	composeService := configKey
	var profiles, initDeps []string

	if !devMode {
		for _, phase := range []string{"Linting", "Unit Tests", "Integration Tests"} {
			key := phaseDependencyKey(projectConfig, phase, configKey)
			if key == "" {
				continue
			}
			if deps := projectConfig.Dependencies[key]; len(deps) > 0 {
				profiles = []string{"--profile", key}
				composeService = deps[len(deps)-1]
				initDeps = append([]string(nil), deps[:len(deps)-1]...)
				foundService = true
				break
			}
		}
		if len(profiles) == 0 {
			// Also match direct compose service names referenced inside [test.dependencies].
			depKeys := make([]string, 0, len(projectConfig.Dependencies))
			for key := range projectConfig.Dependencies {
				depKeys = append(depKeys, key)
			}
			sort.Strings(depKeys)
			for _, key := range depKeys {
				deps := projectConfig.Dependencies[key]
				if len(deps) > 0 && deps[len(deps)-1] == requested {
					profiles = []string{"--profile", key}
					composeService = requested
					initDeps = append([]string(nil), deps[:len(deps)-1]...)
					foundService = true
					break
				}
			}
		}
	}

	if !foundService && len(projectConfig.Services) > 0 {
		return resolvedExecTarget{}, fmt.Errorf("exec service %q is not configured in leedevkit.toml", requested)
	}

	workdir, err := resolveServiceWorkdir(root, configKey, service, workdirOverride)
	if err != nil {
		return resolvedExecTarget{}, err
	}

	if devMode {
		return resolvedExecTarget{
			ComposeService: composeService,
			Workdir:        workdir,
			Files:          devComposeFiles(root),
		}, nil
	}

	files, err := testComposeFiles(root)
	if err != nil {
		return resolvedExecTarget{}, err
	}
	if len(profiles) == 0 {
		customCompose := filepath.Join(root, ".compose", "docker-compose.test.yml")
		if !fileExists(customCompose) && (service.Language == "go" || requested == "go" || len(projectConfig.Services) == 0) {
			profiles = []string{"--profile", "go"}
			if composeService == "" {
				composeService = "go"
			}
		}
	}
	return resolvedExecTarget{
		ComposeService: composeService,
		Workdir:        workdir,
		Profiles:       profiles,
		Dependencies:   initDeps,
		Files:          files,
	}, nil
}

func isComposeServiceReady(ctx context.Context, compose []string, project string, files []string, service, root string, env []string, runner func(context.Context, process.Request) process.Result) bool {
	args := append(composePSJSONArgs(compose, project, files), service)
	var stdout, stderr bytes.Buffer
	res := runner(ctx, process.Request{Args: args, Dir: root, Env: env, Stdout: &stdout, Stderr: &stderr, Timeout: 10 * time.Second})
	if res.ExitCode != 0 {
		return false
	}
	out := res.Stdout
	if len(out) == 0 {
		out = stdout.Bytes()
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return false
	}
	// Check running & healthy using the same Compose ps JSON semantics as lifecycle.Readiness.
	readiness := lifecycle.Readiness{
		Run: func(_ context.Context, _ process.Request) process.Result {
			return process.Result{ExitCode: 0, Stdout: out}
		},
	}
	return readiness.Wait(ctx, args[:len(args)-1], []string{service}, 20*time.Millisecond) == nil
}

func runExecCLI(args []string, root string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runExecContext(ctx, args, root, os.Environ(), stdin, stdout, stderr, process.Run)
}

func runExecContext(ctx context.Context, args []string, root string, rawEnv []string, stdin io.Reader, stdout, stderr io.Writer, runner func(context.Context, process.Request) process.Result) int {
	opts, err := parseExecArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if opts.Help {
		fmt.Fprint(stdout, execUsage)
		return 0
	}
	if runner == nil {
		runner = process.Run
	}
	projectConfig, err := config.Load(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		fmt.Fprintf(stderr, "invalid project config: %v\n", err)
		return 2
	}
	envMap := environMap(rawEnv)
	providerConfig, err := resolveProviderForProject(root, envMap)
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	target, err := resolveExecTarget(root, projectConfig, opts.Service, opts.Dev, opts.Workdir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var project string
	var childEnv []string
	if opts.Dev {
		project = devProjectName(envMap, projectConfig)
		childEnv = withDevProjectEnv(rawEnv, root, project, projectConfig.Namespace)
	} else {
		project = resolveTestProjectName(envMap, projectConfig, root)
		childEnv = withTestProjectEnv(rawEnv, root, project, projectConfig.Namespace)
	}

	alreadyRunning := isComposeServiceReady(ctx, providerConfig.Compose, project, target.Files, target.ComposeService, root, childEnv, runner)
	if !alreadyRunning {
		if opts.Dev {
			upArgs := append(composeBaseArgs(providerConfig.Compose, project, target.Files), "up", "-d", target.ComposeService)
			upRes := runner(ctx, process.Request{Args: upArgs, Dir: root, Env: childEnv, Stdout: stderr, Stderr: stderr})
			if upRes.ExitCode != 0 {
				fmt.Fprintf(stderr, "exec dev startup failed: %s\n", upRes.Diagnostic())
				return 1
			}
			if waitErr := waitComposeServiceReady(ctx, runner, providerConfig.Compose, project, target.Files, target.ComposeService, root, childEnv, 60*time.Second); waitErr != nil {
				fmt.Fprintf(stderr, "exec dev readiness failed: %v\n", waitErr)
				return 1
			}
		} else {
			owned := !ownership.IsLocked(project)
			session, lockErr := ownership.Open(project, owned)
			if lockErr == nil {
				defer session.Close(nil)
			}
			plan := lifecycle.Plan{
				Provider:     providerConfig,
				Project:      project,
				Files:        target.Files,
				Profiles:     target.Profiles,
				Services:     []string{target.ComposeService},
				Dependencies: target.Dependencies,
				OwnsProject:  true,
			}
			upArgs, upErr := plan.Up()
			downArgs, downErr := plan.Down()
			if upErr != nil || downErr != nil {
				fmt.Fprintf(stderr, "invalid exec lifecycle plan: %v\n", firstNonNil(upErr, downErr))
				return 2
			}
			defer func() {
				_ = (lifecycle.ComposeExecutor{Run: runner, Dir: root, Env: childEnv, Stdout: io.Discard, Stderr: io.Discard}).Execute(context.WithoutCancel(ctx), downArgs)
			}()
			ensureBuiltInGoCacheVolumes(ctx, providerConfig.CLI, target.Files, root, childEnv, runner)
			if execErr := (lifecycle.ComposeExecutor{Run: runner, Dir: root, Env: childEnv, Stdout: stderr, Stderr: stderr}).Execute(ctx, upArgs); execErr != nil {
				fmt.Fprintf(stderr, "exec container startup failed: %s\n", process.RedactDiagnostic(execErr.Error()))
				return 1
			}
			if waitErr := waitComposeServiceReady(ctx, runner, providerConfig.Compose, project, target.Files, target.ComposeService, root, childEnv, 120*time.Second); waitErr != nil {
				fmt.Fprintf(stderr, "exec container readiness failed: %v\n", waitErr)
				return 1
			}
		}
	}

	execArgs := composeBaseArgs(providerConfig.Compose, project, target.Files)
	execArgs = append(execArgs, "exec", "-T", "-w", target.Workdir, target.ComposeService)
	execArgs = append(execArgs, opts.Command...)

	res := runner(ctx, process.Request{
		Args:   execArgs,
		Dir:    root,
		Env:    childEnv,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	return res.ExitCode
}

func firstNonNil(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
