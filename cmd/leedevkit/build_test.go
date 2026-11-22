package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/provider"
)

func TestBuildFlowForServiceBuildsFromConfiguredInputs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services: {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM scratch"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := []string{}
	build, fingerprint, err := buildFlowForService(root, "test-project", lifecycle.Plan{
		Provider: provider.Provider{Name: "docker", Compose: []string{"docker", "compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps", "build"}},
		Project:  "test-project",
	}, config.Service{Image: "test-image", Dockerfile: "Dockerfile", ComposeFile: "compose.yml"}, lifecycle.BuildRunner{
		Execute: func(_ context.Context, args []string) error {
			if !contains(args, "build") {
				t.Fatalf("build args = %#v", args)
			}
			calls = append(calls, "build")
			return nil
		},
		Verify: func(_ context.Context, engine, image string) error {
			calls = append(calls, "verify:"+engine+":"+image)
			return nil
		},
		Publish: func(metadata lifecycle.BuildMetadata) error {
			calls = append(calls, "publish:"+metadata.Expected)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	if err := build(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[0] != "build" || calls[1] != "verify:docker:test-image" || calls[2] != "publish:"+fingerprint {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestGlobalComposeAssetsConfigFromUnrelatedCWDWithSpacedProjectRoot(t *testing.T) {
	compose, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker CLI unavailable")
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(sourceFile), "../.."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	projectRoot := filepath.Join(fixture, "project root")
	cwd := filepath.Join(fixture, "unrelated cwd")
	for _, path := range []string{projectRoot, cwd} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, asset := range []string{"go", "rust"} {
		composeFile := filepath.Join("container", asset, "docker-compose.test.yml")
		command := exec.Command(compose, "compose", "--profile", asset, "-f", filepath.Join(repoRoot, composeFile), "config", "--format", "json")
		command.Dir = cwd
		command.Env = append(os.Environ(), "LEEDEVKIT_PROJECT_ROOT="+projectRoot)
		output, err := command.Output()
		if err != nil {
			t.Fatalf("docker compose config for %s: %v", asset, err)
		}
		if !strings.Contains(string(output), filepath.Join(repoRoot, "container", asset)) || !strings.Contains(string(output), projectRoot) {
			t.Fatalf("resolved config for %s missing asset context or project mount: %s", asset, output)
		}
	}
}

func TestCustomComposeWithGlobalAssetSuffixKeepsProjectContext(t *testing.T) {
	root := t.TempDir()
	compose := filepath.Join(root, "custom", "container", "go", "docker-compose.test.yml")
	if err := os.MkdirAll(filepath.Dir(compose), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		compose:                           "services: {}",
		filepath.Join(root, "Dockerfile"): "FROM scratch",
		filepath.Join(root, "custom", "Dockerfile"): "FROM scratch",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	service := config.Service{Image: "image", Dockerfile: "Dockerfile", ComposeFile: compose}
	_, before, err := buildFlowForService(root, "project", lifecycle.Plan{}, service, lifecycle.BuildRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "custom", "project-input.txt"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, after, err := buildFlowForService(root, "project", lifecycle.Plan{}, service, lifecycle.BuildRunner{})
	if err != nil || before == after {
		t.Fatalf("custom Compose context was not fingerprinted: %v", err)
	}
}

func TestGlobalComposeAssetsFingerprintOnlySelectedBuildContext(t *testing.T) {
	for _, path := range []string{
		filepath.Join("container", "go", "docker-compose.test.yml"),
		filepath.Join(".leedevkit", "container", "rust", "docker-compose.test.yml"),
	} {
		root := t.TempDir()
		assetDir := filepath.Join(root, filepath.Dir(path))
		if err := os.MkdirAll(assetDir, 0o700); err != nil {
			t.Fatal(err)
		}
		for name, content := range map[string]string{
			"docker-compose.test.yml": "services: {}",
			"Dockerfile":              "FROM scratch",
		} {
			if err := os.WriteFile(filepath.Join(assetDir, name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		composePath, err := filepath.Abs(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		service := config.Service{Image: "image", Dockerfile: ".leedevkit/container/rust/Dockerfile", ComposeFile: composePath}
		_, before, err := buildFlowForService(root, "project", lifecycle.Plan{}, service, lifecycle.BuildRunner{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("project source"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, unchanged, err := buildFlowForService(root, "project", lifecycle.Plan{}, service, lifecycle.BuildRunner{})
		if err != nil || before != unchanged {
			t.Fatalf("unrelated project source changed fingerprint for %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, "Dockerfile"), []byte("FROM scratch\n# changed"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, changed, err := buildFlowForService(root, "project", lifecycle.Plan{}, service, lifecycle.BuildRunner{})
		if err != nil || before == changed {
			t.Fatalf("asset Dockerfile did not change fingerprint for %s: %v", path, err)
		}
	}
}

func TestBuildFlowForServiceRejectsPartialBuildConfig(t *testing.T) {
	_, _, err := buildFlowForService(t.TempDir(), "test-project", lifecycle.Plan{}, config.Service{Dockerfile: "Dockerfile"}, lifecycle.BuildRunner{})
	if err == nil {
		t.Fatal("expected partial build config error")
	}
}

func TestBuildCallbackRunsBuildBeforePhaseLifecycle(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"compose.yml": "services: {}", "Dockerfile": "FROM scratch"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	calls := []string{}
	plan := lifecycle.Plan{
		Provider: provider.Provider{Name: "docker", CLI: []string{"docker"}, Compose: []string{"docker", "compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps", "build"}},
		Project:  "test-project",
	}
	callback, selected, err := buildCallbackForService(root, "test-project", plan, config.Service{Image: "test-image", Dockerfile: "Dockerfile", ComposeFile: "compose.yml"}, "phase", false, lifecycle.BuildRunner{
		Execute: func(context.Context, []string) error { calls = append(calls, "build"); return nil },
		Verify:  func(context.Context, string, string) error { calls = append(calls, "verify"); return nil },
		Publish: func(lifecycle.BuildMetadata) error { calls = append(calls, "publish"); return nil },
	})
	if err != nil || !selected || callback == nil {
		t.Fatalf("callback=%v selected=%v err=%v", callback != nil, selected, err)
	}
	if err := callback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[0] != "build" || calls[1] != "verify" || calls[2] != "publish" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestBuildCallbackBuildsEveryPhaseForPhasePolicy(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"compose.yml": "services: {}", "Dockerfile": "FROM scratch"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	builds := 0
	plan := lifecycle.Plan{
		Provider: provider.Provider{Name: "docker", CLI: []string{"docker"}, Capabilities: []string{"compose", "config", "up", "down", "ps", "build"}},
		Project:  "test-project",
	}
	service := config.Service{Image: "test-image", Dockerfile: "Dockerfile", ComposeFile: "compose.yml"}
	callback, selected, err := buildCallbackForService(root, "test-project", plan, service, "phase", false, lifecycle.BuildRunner{
		Execute: func(context.Context, []string) error { builds++; return nil },
		Verify:  func(context.Context, string, string) error { return nil },
		Publish: lifecycle.MetadataPublisher(root, "test-project"),
	})
	if err != nil || !selected || callback == nil {
		t.Fatalf("callback=%v selected=%v err=%v", callback != nil, selected, err)
	}
	if err := callback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := callback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if builds != 2 {
		t.Fatalf("build count = %d, want 2", builds)
	}
}

func TestBuildCallbackBuildsOnceForOncePolicy(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"compose.yml": "services: {}", "Dockerfile": "FROM scratch"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	builds := 0
	plan := lifecycle.Plan{
		Provider: provider.Provider{Name: "docker", CLI: []string{"docker"}, Capabilities: []string{"compose", "config", "up", "down", "ps", "build"}},
		Project:  "test-project",
	}
	service := config.Service{Image: "test-image", Dockerfile: "Dockerfile", ComposeFile: "compose.yml"}
	runner := lifecycle.BuildRunner{
		Execute: func(context.Context, []string) error { builds++; return nil },
		Verify:  func(context.Context, string, string) error { return nil },
		Publish: lifecycle.MetadataPublisher(root, "test-project"),
	}
	callback, selected, err := buildCallbackForService(root, "test-project", plan, service, "once", false, runner)
	if err != nil || !selected || callback == nil {
		t.Fatalf("callback=%v selected=%v err=%v", callback != nil, selected, err)
	}
	if err := callback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := callback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if builds != 1 {
		t.Fatalf("build count = %d, want 1", builds)
	}
}
