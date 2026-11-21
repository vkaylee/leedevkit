package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDependencies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	content := `[test.dependencies]
int-go = ["postgres", "redis"]
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Dependencies["int-go"]) != 2 {
		t.Fatalf("dependencies = %#v", project.Dependencies)
	}
}
