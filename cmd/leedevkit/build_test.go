package main

import (
	"context"
	"os"
	"path/filepath"
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

func TestBuildFlowForServiceRejectsPartialBuildConfig(t *testing.T) {
	_, _, err := buildFlowForService(t.TempDir(), "test-project", lifecycle.Plan{}, config.Service{Dockerfile: "Dockerfile"}, lifecycle.BuildRunner{})
	if err == nil {
		t.Fatal("expected partial build config error")
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
