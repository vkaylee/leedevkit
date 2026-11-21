package run

import (
	"reflect"
	"testing"
)

func TestPlanGoRunForIsolatedProject(t *testing.T) {
	got, err := Plan(Request{
		Compose: []string{"docker", "compose"},
		Project: "leedevkit-test-abc",
		Files:   []string{"/workspace/container/go/docker-compose.test.yml"},
		Tool:    "go",
		Args:    []string{"test", "./..."},
		Service: "go",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker", "compose", "-p", "leedevkit-test-abc", "--profile", "go",
		"-f", "/workspace/container/go/docker-compose.test.yml",
		"run", "-T", "--rm", "--no-deps", "--entrypoint", "go", "go", "test", "./...",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestPlanCargoTestAddsDatabaseProfilesAndRewritesNextest(t *testing.T) {
	got, err := Plan(Request{
		Compose:  []string{"podman-compose"},
		Project:  "test",
		Files:    []string{"api.yml"},
		Profiles: nil,
		Tool:     "cargo",
		Args:     []string{"test", "auth"},
		Service:  "apiserver",
		Workdir:  "/workspace/api",
		Pooler:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"podman-compose", "-p", "test", "--profile", "int-api",
		"--profile", "infra-db", "--profile", "infra-redis", "--profile", "infra-pooler",
		"-f", "api.yml", "run", "-T", "--rm", "--workdir", "/workspace/api",
		"--entrypoint", "cargo", "apiserver", "nextest", "run", "--test-threads", "4", "auth",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestPlanNpmRunningServiceUsesExecAndRewritesTypecheck(t *testing.T) {
	got, err := Plan(Request{
		Compose: []string{"docker", "compose"},
		Project: "dev",
		Files:   []string{"docker-compose.yml", "dev.yml"},
		Tool:    "npm",
		Args:    []string{"run", "typecheck"},
		Service: "webdashboard",
		Running: true,
		Dev:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker", "compose", "-p", "dev", "--profile", "frontend",
		"-f", "docker-compose.yml", "-f", "dev.yml",
		"exec", "-T", "webdashboard", "bun", "run", "type-check",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestPlanRejectsUnsafeArguments(t *testing.T) {
	_, err := Plan(Request{
		Compose: []string{"docker", "compose"}, Project: "test", Tool: "go",
		Args: []string{"-c", "$(id)"}, Service: "go",
	})
	if err == nil {
		t.Fatal("expected unsafe argument error")
	}
}
