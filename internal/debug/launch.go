package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type ComposeRunner func(context.Context, []string) error

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

func Launch(ctx context.Context, environment Environment, compose []string, run ComposeRunner) error {
	override := filepath.Join(environment.ProjectRoot, ".leedevkit-debug.override.yml")
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
