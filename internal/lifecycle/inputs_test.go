package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBuildInputs(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "compose.yml"), filepath.Join(dir, "Dockerfile"), filepath.Join(dir, "context")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	compose, docker, context, err := ReadBuildInputs(paths[0], paths[1], paths[2])
	if err != nil || string(compose) != paths[0] || string(docker) != paths[1] || string(context) != paths[2] {
		t.Fatalf("inputs = %q %q %q err=%v", compose, docker, context, err)
	}
}
