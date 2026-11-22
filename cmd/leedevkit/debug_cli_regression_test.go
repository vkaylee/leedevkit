package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugCLIDirectServesFixtureAndRejectsInvalidManifest(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	data := `[debug]
manifest_version = 1
[debug.environments.fixture]
compose_project = 'fixture-project'
`
	if err := os.WriteFile(manifest, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("LEEDEVKIT_DEBUG_DIRECT", "1")
	var stdout, stderr bytes.Buffer
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n")
	if code := run([]string{"debug", "mcp", "--environment", "fixture", "--manifest", manifest}, input, &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	var response map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || response["result"] == nil {
		t.Fatalf("response=%q error=%v", stdout.String(), err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"debug", "mcp", "--environment", "production", "--manifest", manifest}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "production") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}
