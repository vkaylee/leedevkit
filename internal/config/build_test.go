package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServiceBuildInputs(t *testing.T) {
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
	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	service := project.Services["go"]
	if service.Dockerfile != "container/go/Dockerfile" || service.ComposeFile != "container/go/docker-compose.test.yml" {
		t.Fatalf("service = %#v", service)
	}
}
