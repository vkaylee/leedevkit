package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugMainSubprocess(t *testing.T) {
	if os.Getenv("DEBUG_MAIN_HELPER") == "1" {
		os.Args = append([]string{"debug-mcp"}, strings.Split(os.Getenv("DEBUG_MAIN_ARGS"), "\n")...)
		main()
		return
	}
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(manifest, []byte("[debug]\nmanifest_version = 1\n[debug.environments.fixture]\ncompose_project = 'fixture-project'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		wantErr  string
	}{
		{"valid", []string{"--environment", "fixture", "--manifest", manifest, "--project-root", root}, 0, ""},
		{"missing manifest", []string{"--environment", "fixture"}, 2, "required"},
		{"unknown environment", []string{"--environment", "missing", "--manifest", manifest}, 2, "unknown or unsafe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestDebugMainSubprocess$")
			cmd.Env = append(os.Environ(), "DEBUG_MAIN_HELPER=1", "DEBUG_MAIN_ARGS="+strings.Join(tc.args, "\n"))
			cmd.Stdin = strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if tc.wantCode == 0 {
				var response map[string]any
				if err != nil || json.NewDecoder(&stdout).Decode(&response) != nil || response["result"] == nil {
					t.Fatalf("error=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
				}
				return
			}
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != tc.wantCode || !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("error=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}
