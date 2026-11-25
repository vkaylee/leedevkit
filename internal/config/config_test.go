package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadProjectReadsServicesTargetsAndTestSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	content := `[services.go]
lang = "go"
workdir = "backend"
[services.web]
lang = "typescript"
[targets]
go = ["go"]
all = ["go", "web"]
[test]
build_policy = "phase"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Services["go"].Language != "go" || got.Services["go"].Workdir != "backend" {
		t.Fatalf("services = %#v", got.Services)
	}
	if !reflect.DeepEqual(got.Targets["all"], []string{"go", "web"}) {
		t.Fatalf("targets = %#v", got.Targets)
	}
	if got.BuildPolicy != "phase" {
		t.Fatalf("build policy = %q", got.BuildPolicy)
	}
}

func TestLoadMissingProjectUsesDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got.BuildPolicy != "once" {
		t.Fatalf("build policy = %q", got.BuildPolicy)
	}
}

func TestLoadIgnoresLegacyServiceImageAndComposeFileKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	content := `[services.go]
lang = "go"
dockerfile = "container/go/Dockerfile"
compose_file = "container/go/docker-compose.test.yml"
image = "project-go"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("legacy keys rejected: %v", err)
	}
	if got.Services["go"].Dockerfile != "container/go/Dockerfile" {
		t.Fatalf("service = %#v", got.Services["go"])
	}
}

func TestLoadNamespaceDefaultIntegrationAndExplicitGateOverrides(t *testing.T) {
	t.Run("default go integration gate from int-api dependency", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "leedevkit.toml")
		content := `[project]
name = "Shop"
namespace = "shop"
languages = ["go", "typescript"]

[services.apiserver]
lang = "go"
go = true
workdir = "apiserver"
ports = [8080]
health_check = "/health"

[targets]
api = ["apiserver"]
all = ["apiserver"]

[test.dependencies]
int-api = ["postgres", "redis", "apiserver"]
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Namespace != "shop" || got.Name != "Shop" {
			t.Fatalf("project metadata = %q / %q", got.Name, got.Namespace)
		}
		intGates := got.GatesForService("apiserver", "integration")
		if len(intGates) != 1 || !reflect.DeepEqual(intGates[0].Command, []string{"go", "test", "-tags=integration", "./..."}) {
			t.Fatalf("default intGates = %#v", intGates)
		}
	})

	t.Run("explicit [test.gates.int-api] override", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "leedevkit.toml")
		content := `[services.apiserver]
lang = "go"
[targets]
api = ["apiserver"]
[test.dependencies]
int-api = ["postgres", "apiserver"]
[test.gates.int-api]
command = ["go", "test", "-tags=integration", "-count=1", "./cmd/..."]
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		intGates := got.GatesForService("apiserver", "integration")
		if len(intGates) != 1 || !reflect.DeepEqual(intGates[0].Command, []string{"go", "test", "-tags=integration", "-count=1", "./cmd/..."}) {
			t.Fatalf("override intGates = %#v", intGates)
		}
		if len(got.GatesForService("apiserver", "unit")) != 1 {
			t.Fatalf("unit default lost: %#v", got.GatesForService("apiserver", "unit"))
		}
	})

	t.Run("explicit [services.apiserver.test] single and multi command overrides", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "leedevkit.toml")
		content := `[services.apiserver]
go = true
[services.apiserver.test]
lint = [["gofmt", "-l", "."], ["golangci-lint", "run"]]
integration = ["go", "test", "-tags=integration", "-race", "./..."]
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Services["apiserver"].Language != "go" {
			t.Fatalf("go=true did not infer lang=go: %#v", got.Services["apiserver"])
		}
		if lint := got.GatesForService("apiserver", "lint"); len(lint) != 2 || lint[1].Command[0] != "golangci-lint" {
			t.Fatalf("lint overrides = %#v", lint)
		}
		if integ := got.GatesForService("apiserver", "integration"); len(integ) != 1 || integ[0].Command[3] != "-race" {
			t.Fatalf("integration override = %#v", integ)
		}
	})

	t.Run("rejects invalid namespace, empty dependencies, and bad gate definitions", func(t *testing.T) {
		for _, snippet := range []string{
			"[project]\nnamespace = '../bad'\n",
			"[test.dependencies]\nint-api = []\n",
			"[test.gates.unknown_name]\ncommand = ['go', 'test']\n",
			"[services.api]\nlang = 'go'\n[services.api.test]\nintegration = []\n",
		} {
			path := filepath.Join(t.TempDir(), "bad.toml")
			if err := os.WriteFile(path, []byte(snippet), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatalf("expected error for snippet:\n%s", snippet)
			}
		}
	})
}
