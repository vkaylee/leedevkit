package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type BuildFlow struct{ Runner BuildRunner }

func (f BuildFlow) Run(ctx context.Context, plan Plan, fingerprint, engine, image string) error {
	args, err := plan.BuildArgs()
	if err != nil {
		return err
	}
	metadata := BuildMetadata{Expected: fingerprint}
	runner := f.Runner
	if runner.Verify == nil {
		runner.Verify = func(ctx context.Context, engine, image string) error {
			return VerifyImageFingerprint(ctx, nil, engine, image, fingerprint)
		}
	}
	runner.Engine, runner.Image = engine, image
	return runner.Run(ctx, fingerprint, args, metadata)
}

func (f BuildFlow) RunFromFiles(ctx context.Context, plan Plan, root, composeFile, dockerfile, contextRoot, engine, image string) error {
	compose, docker, buildContext, err := ReadBuildInputs(
		filepath.Join(root, composeFile),
		filepath.Join(root, dockerfile),
		filepath.Join(root, contextRoot),
	)
	if err != nil {
		return fmt.Errorf("read build inputs: %w", err)
	}
	fingerprint := BuildFingerprintInputs(compose, docker, buildContext)
	return f.Run(ctx, plan, fingerprint, engine, image)
}

func MetadataPublisher(root, project string) func(BuildMetadata) error {
	return func(metadata BuildMetadata) error {
		return WriteMetadata(root, project, metadata)
	}
}

func BuildContextPath(root string) string {
	if root == "" {
		return os.Getenv("PWD")
	}
	return root
}
