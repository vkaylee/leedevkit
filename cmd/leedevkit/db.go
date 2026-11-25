package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/dbcommands"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

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

func runCoverageCLI(args []string, root string, stdin io.Reader, stdout, stderr io.Writer) int {
	providerConfig, err := resolveProviderForProject(root, environMap(os.Environ()))
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	projectConfig, err := config.Load(filepath.Join(root, "leedevkit.toml"))
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
	return runCommand(root, root, coverageArgs, stdin, stdout, stderr)
}

func runGeneratedCLI(args []string, root string, stdin io.Reader, stdout, stderr io.Writer) int {
	providerConfig, err := resolveProviderForProject(root, environMap(os.Environ()))
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	projectConfig, err := config.Load(filepath.Join(root, "leedevkit.toml"))
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
		if code := runCommand(root, root, argv, stdin, stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}

func runAPISyncCLI(root string, stdin io.Reader, stdout, stderr io.Writer) int {
	providerConfig, err := resolveProviderForProject(root, environMap(os.Environ()))
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	projectConfig, err := config.Load(filepath.Join(root, "leedevkit.toml"))
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
		if code := runCommand(root, root, command, stdin, stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}

func runMigrateCLI(args []string, root string, stdout, stderr io.Writer) int {
	action, err := parseMigrationCommand(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	env := environMap(os.Environ())
	providerConfig, err := resolveProviderForProject(root, env)
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	projectConfig, loadErr := config.Load(filepath.Join(root, "leedevkit.toml"))
	project := strings.TrimSpace(env["LEEDEVKIT_TEST_PROJECT"])
	if project == "" {
		project = strings.TrimSpace(env["COMPOSE_PROJECT_NAME"])
	}
	if project == "" {
		if loadErr == nil && projectConfig.Namespace != "" {
			project = projectConfig.Namespace + "-test"
		} else {
			project = "leedevkit-test"
		}
	}
	service := "apiserver"
	if loadErr == nil {
		service = configuredServiceName(projectConfig, "rust")
	}
	migrationArgs, err := buildMigrationCommand(root, providerConfig, project, service, action)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	result := process.Run(context.Background(), process.Request{Args: migrationArgs, Dir: root, Stdout: stdout, Stderr: stderr})
	return result.ExitCode
}

func runDBQueryCLI(args []string, root string, stdout, stderr io.Writer) int {
	sql, asJSON, err := parseDatabaseQueryArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	env := environMap(os.Environ())
	providerConfig, err := resolveProviderForProject(root, env)
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	projectConfig, _ := config.Load(filepath.Join(root, "leedevkit.toml"))
	project := devProjectName(env, projectConfig)
	queryArgs, err := buildDatabaseQueryArgs(providerConfig, project, sql, asJSON)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	result := process.Run(context.Background(), process.Request{Args: queryArgs, Dir: root, Stdout: stdout, Stderr: stderr})
	return result.ExitCode
}
