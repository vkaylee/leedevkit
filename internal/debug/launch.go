package debug

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vkaylee/leedevkit/internal/process"
)

type ComposeRunner func(context.Context, []string) error

type ComposeOutputRunner func(context.Context, []string) ([]byte, error)

func ProcessComposeRunner(stdout, stderr io.Writer) ComposeRunner {
	return func(ctx context.Context, args []string) error {
		result := process.Run(ctx, process.Request{Args: args, Stdout: stdout, Stderr: stderr})
		if result.ExitCode != 0 {
			if result.Err != nil {
				return result.Err
			}
			return fmt.Errorf("Compose exited with code %d", result.ExitCode)
		}
		return nil
	}
}

func ProcessComposeOutputRunner() ComposeOutputRunner {
	return func(ctx context.Context, args []string) ([]byte, error) {
		var stdout, stderr bytes.Buffer
		result := process.Run(ctx, process.Request{Args: args, Stdout: &stdout, Stderr: &stderr})
		if result.ExitCode != 0 {
			if result.Err != nil {
				return nil, result.Err
			}
			return nil, fmt.Errorf("Compose exited with code %d: %s", result.ExitCode, stderr.String())
		}
		return stdout.Bytes(), nil
	}
}

func ComposeBaseArgs(compose []string, environment Environment) ([]string, error) {
	if len(compose) == 0 || environment.ComposeProject == "" {
		return nil, fmt.Errorf("compose command and project are required")
	}
	args := append([]string{}, compose...)
	args = append(args, "-p", environment.ComposeProject)
	for _, file := range environment.ComposeFiles {
		args = append(args, "-f", filepath.Join(environment.ProjectRoot, file))
	}
	for _, profile := range environment.Profiles {
		args = append(args, "--profile", profile)
	}
	return args, nil
}

func ValidateComposeConfig(config map[string]any, environment Environment) error {
	services, ok := config["services"].(map[string]any)
	if !ok || len(services) == 0 {
		return fmt.Errorf("Compose config has no services")
	}
	networks, ok := config["networks"].(map[string]any)
	if !ok {
		return fmt.Errorf("Compose config has no networks")
	}
	for _, approved := range environment.Networks {
		if _, ok := networks[approved]; !ok {
			return fmt.Errorf("approved network is not in selected Compose project: %s", approved)
		}
	}
	for _, service := range environment.Services {
		if _, ok := services[service.Host]; ok {
			continue
		}
		found := false
		for _, alias := range service.Aliases {
			if _, ok := services[alias]; ok {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("approved service is not in selected Compose project: %s", service.ID)
		}
	}
	return nil
}

func BuildComposeConfigArgs(compose []string, environment Environment) ([]string, error) {
	args, err := ComposeBaseArgs(compose, environment)
	if err != nil {
		return nil, err
	}
	return append(args, "config", "--format", "json"), nil
}

func ParseComposeConfig(data []byte) (map[string]any, error) {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil || config == nil {
		return nil, fmt.Errorf("Compose config output is not valid JSON")
	}
	return config, nil
}

func ResolveNetworkNames(config map[string]any, environment Environment) ([]string, error) {
	networks, ok := config["networks"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Compose config has no networks")
	}
	result := make([]string, 0, len(environment.Networks))
	for _, approved := range environment.Networks {
		network, ok := networks[approved].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("approved network is not in selected Compose project: %s", approved)
		}
		name := environment.ComposeProject + "_" + approved
		if configured, ok := network["name"].(string); ok && configured != "" {
			name = configured
		}
		result = append(result, name)
	}
	return result, nil
}

func BuildNetworkInspectArgs(engine []string, network string) ([]string, error) {
	if len(engine) == 0 || network == "" {
		return nil, fmt.Errorf("engine and network are required")
	}
	return append(append([]string{}, engine...), "network", "inspect", network), nil
}

func ValidateNetworkOwner(data []byte, project string) error {
	var inspected []struct {
		Labels map[string]string `json:"Labels"`
	}
	if err := json.Unmarshal(data, &inspected); err != nil || len(inspected) == 0 {
		return fmt.Errorf("network inspection output is invalid")
	}
	if inspected[0].Labels["com.docker.compose.project"] != project {
		return fmt.Errorf("network is not owned by selected Compose project")
	}
	return nil
}

func BuildDebugRunArgs(compose []string, environment Environment, override string) ([]string, error) {
	args, err := ComposeBaseArgs(compose, environment)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(environment.ProjectRoot, environment.SourcePath)
	if err != nil || relative == ".." || len(relative) > 2 && relative[:3] == ".."+string(filepath.Separator) {
		return nil, fmt.Errorf("manifest must stay inside project root")
	}
	args = append(args, "-f", override, "--profile", "debug", "run", "--rm", "-T", "--no-deps", "--build", "leedevkit-debug", "--environment", environment.Name, "--manifest", "/workspace/"+filepath.ToSlash(relative), "--project-root", "/workspace")
	return args, nil
}

func BuildDebugCleanupArgs(compose []string, environment Environment, override string) ([]string, error) {
	args, err := ComposeBaseArgs(compose, environment)
	if err != nil {
		return nil, err
	}
	return append(args, "-f", override, "rm", "--force", "--stop", "--no-deps", "leedevkit-debug"), nil
}

func LaunchWithValidation(ctx context.Context, environment Environment, compose, engine []string, run ComposeRunner, output ComposeOutputRunner) error {
	configArgs, err := BuildComposeConfigArgs(compose, environment)
	if err != nil {
		return err
	}
	configOutput, err := output(ctx, configArgs)
	if err != nil {
		return err
	}
	config, err := ParseComposeConfig(configOutput)
	if err != nil {
		return err
	}
	if err := ValidateComposeConfig(config, environment); err != nil {
		return err
	}
	networks, err := ResolveNetworkNames(config, environment)
	if err != nil {
		return err
	}
	for _, network := range networks {
		inspect, err := BuildNetworkInspectArgs(engine, network)
		if err != nil {
			return err
		}
		ownerOutput, err := output(ctx, inspect)
		if err != nil {
			return err
		}
		if err := ValidateNetworkOwner(ownerOutput, environment.ComposeProject); err != nil {
			return err
		}
	}
	return launchWithNetworks(ctx, environment, compose, run, networks)
}

func LaunchValidated(ctx context.Context, environment Environment, compose, engine []string, run ComposeRunner, configOutput, networkOutput []byte) error {
	config, err := ParseComposeConfig(configOutput)
	if err != nil {
		return err
	}
	if err := ValidateComposeConfig(config, environment); err != nil {
		return err
	}
	networks, err := ResolveNetworkNames(config, environment)
	if err != nil {
		return err
	}
	for _, network := range networks {
		inspect, err := BuildNetworkInspectArgs(engine, network)
		if err != nil {
			return err
		}
		_ = inspect
		if err := ValidateNetworkOwner(networkOutput, environment.ComposeProject); err != nil {
			return err
		}
	}
	return launchWithNetworks(ctx, environment, compose, run, networks)
}

func Launch(ctx context.Context, environment Environment, compose []string, run ComposeRunner) error {
	networks := append([]string(nil), environment.Networks...)
	if len(networks) == 0 {
		networks = []string{"debug"}
	}
	return launchWithNetworks(ctx, environment, compose, run, networks)
}

func launchWithNetworks(ctx context.Context, environment Environment, compose []string, run ComposeRunner, networks []string) error {
	directory, err := os.MkdirTemp("", "leedevkit-debug-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	image := "leedevkit-debug:go"
	override, err := WriteOverride(environment, networks, directory, image)
	if err != nil {
		return err
	}
	runArgs, err := BuildDebugRunArgs(compose, environment, override)
	if err != nil {
		return err
	}
	cleanupArgs, err := BuildDebugCleanupArgs(compose, environment, override)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("compose runner is required")
	}
	defer func() { _ = run(context.WithoutCancel(ctx), cleanupArgs) }()
	return run(ctx, runArgs)
}
