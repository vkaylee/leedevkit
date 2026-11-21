package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadQualityGatesUsesConfiguredGoDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	content := `[quality.gates.custom]
phase = "lint"
command = ["custom", "check"]
required = false
failure_policy = "optional"

[quality.go]
lint = ["custom"]
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Gates) != 1 || project.Gates[0].Name != "custom" || project.Gates[0].FailurePolicy != "optional" {
		t.Fatalf("gates = %#v", project.Gates)
	}
}

func TestLoadQualityGatesDefaultsToGoGates(t *testing.T) {
	project, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Gates) != 4 || project.Gates[0].Name != "gofmt" {
		t.Fatalf("gates = %#v", project.Gates)
	}
}
