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
