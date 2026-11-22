package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/process"
)

func buildFlowForService(root, project string, plan lifecycle.Plan, service config.Service, runner lifecycle.BuildRunner) (func(context.Context) error, string, error) {
	if service.Dockerfile == "" || service.ComposeFile == "" || service.Image == "" {
		return nil, "", fmt.Errorf("service %q requires image, dockerfile, and compose_file for build", service.Name)
	}
	composePath := service.ComposeFile
	if !filepath.IsAbs(composePath) {
		composePath = filepath.Join(root, composePath)
	}
	dockerfilePath := filepath.Join(root, service.Dockerfile)
	contextRoot := root
	isGlobalAsset := isGlobalComposeAsset(root, composePath) || isGlobalComposeAsset(filepath.Join(root, ".leedevkit"), composePath)
	if !isGlobalAsset {
		assetRoot, err := runtimeAssetRoot()
		if err == nil {
			isGlobalAsset = isGlobalComposeAsset(assetRoot, composePath)
		}
	}
	if isGlobalAsset {
		contextRoot = filepath.Dir(composePath)
		dockerfilePath = filepath.Join(contextRoot, "Dockerfile")
	}
	compose, docker, contextData, err := lifecycle.ReadBuildInputs(composePath, dockerfilePath, contextRoot)
	if err != nil {
		return nil, "", fmt.Errorf("read build inputs: %w", err)
	}
	fingerprint := lifecycle.BuildFingerprintInputs(compose, docker, contextData)
	if runner.Verify == nil {
		runner.Verify = func(ctx context.Context, engine, image string) error {
			return lifecycle.VerifyImageFingerprint(ctx, process.Run, engine, image, fingerprint)
		}
	}
	flow := lifecycle.BuildFlow{Runner: runner}
	return func(ctx context.Context) error {
		return flow.Run(ctx, plan, fingerprint, buildEngine(plan), service.Image)
	}, fingerprint, nil
}

func isGlobalComposeAsset(root, composePath string) bool {
	for _, language := range []string{"go", "rust"} {
		assetPath := filepath.Join(root, "container", language, "docker-compose.test.yml")
		if filepath.Clean(composePath) == filepath.Clean(assetPath) {
			return true
		}
	}
	return false
}

func buildEngine(plan lifecycle.Plan) string {
	if len(plan.Provider.CLI) > 0 {
		return plan.Provider.CLI[0]
	}
	return "docker"
}

func newBuildRunner(root, project string, output io.Writer) lifecycle.BuildRunner {
	return lifecycle.BuildRunner{
		Lock: lifecycle.NewBuildLock(),
		Execute: func(ctx context.Context, args []string) error {
			result := process.Run(ctx, process.Request{Args: args, Dir: root, Env: withProjectRoot(os.Environ(), root), Stdout: output, Stderr: output})
			if result.ExitCode == 0 {
				return nil
			}
			if result.Err != nil {
				return result.Err
			}
			return fmt.Errorf("build exited with code %d", result.ExitCode)
		},
		Publish: lifecycle.MetadataPublisher(root, project),
	}
}

func buildCallbackForService(root, project string, plan lifecycle.Plan, service config.Service, policy string, skip bool, runner lifecycle.BuildRunner) (func(context.Context) error, bool, error) {
	if service.Dockerfile == "" || service.ComposeFile == "" || service.Image == "" || skip {
		return nil, false, nil
	}
	build, fingerprint, err := buildFlowForService(root, project, plan, service, runner)
	if err != nil {
		return nil, false, err
	}
	metadata, err := lifecycle.ReadMetadata(root, project)
	if err != nil {
		return nil, false, fmt.Errorf("read build metadata: %w", err)
	}
	if policy == "once" && metadata.Expected == fingerprint && metadata.Actual == fingerprint {
		return nil, false, nil
	}
	if policy == "phase" {
		return build, true, nil
	}
	var once sync.Once
	var buildErr error
	return func(ctx context.Context) error {
		once.Do(func() { buildErr = build(ctx) })
		return buildErr
	}, true, nil
}
