package debug

import (
	"context"
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

func BuildComposeConfigArgs(compose []string, environment Environment) ([]string, error) {
	args, err := ComposeBaseArgs(compose, environment)
	if err != nil {
		return nil, err
	}
	return append(args, "config"), nil
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
