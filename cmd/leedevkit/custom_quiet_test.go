package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomGateQuietPreservesLog(t *testing.T) {
	for _, flag := range []string{"--quiet", "--json", "--json-stream"} {
		t.Run(flag, func(t *testing.T) {
			root := t.TempDir()
			engine := filepath.Join(root, "compose-fixture")
			script := "#!/bin/sh\nfor arg do\n case \"$arg\" in ps) printf '[{\"Service\":\"backend\",\"State\":\"running\"}]\\n'; exit;; version) printf 'tool 1.0\\n'; exit;; setup|custom|teardown) printf '%s stdout\\n' \"$arg\"; printf '%s stderr\\n' \"$arg\" >&2; if [ \"$arg\" = custom ] && [ \"${FAIL_GATE:-}\" = yes ]; then exit 7; fi; if [ \"$arg\" = custom ] && [ \"${FAIL_GATE:-}\" = timeout ]; then sleep 3; fi; exit;; esac\ndone\nprintf 'lifecycle stdout\\n'\nprintf 'lifecycle stderr\\n' >&2\n"
			if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			config := "[container]\nprovider = 'fixture'\n[container.providers.fixture]\ncli = ['fixture-engine']\ncompose = ['" + engine + "']\n[services.backend]\nlang = 'go'\n[targets]\nall = ['go']\n[quality.gates.custom]\nphase = 'unit'\ncommand = ['custom']\nversion_command = ['version']\nexpected_version = 'tool 1.0'\nsetup_command = ['setup']\nteardown_command = ['teardown']\ntimeout = 0.2\nrequired = true\n[quality.go]\nunit = ['custom']\n[test]\nbuild_policy = 'once'\n"
			if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, ".compose"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			t.Setenv("HOME", root)
			t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
			var stdout, stderr bytes.Buffer
			code := run([]string{"test", "all", "--unit-only", flag}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if strings.Contains(stdout.String(), "custom stdout") || strings.Contains(stderr.String(), "custom stderr") || strings.Contains(stdout.String(), "lifecycle stdout") || strings.Contains(stderr.String(), "lifecycle stderr") {
				t.Fatalf("quiet leaked output: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if flag == "--json" && !json.Valid(stdout.Bytes()) {
				t.Fatalf("invalid JSON: %q", stdout.String())
			}
			if flag == "--json-stream" {
				for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
					if !json.Valid([]byte(line)) {
						t.Fatalf("invalid JSONL: %q", line)
					}
				}
			}
			logs, err := filepath.Glob(filepath.Join(root, ".test_logs", "*", "*.log"))
			if err != nil || len(logs) == 0 {
				t.Fatalf("logs=%v err=%v", logs, err)
			}
			found := false
			for _, path := range logs {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(data, []byte("custom stdout")) && bytes.Contains(data, []byte("custom stderr")) {
					for _, marker := range []string{"tool 1.0", "setup stdout", "setup stderr", "teardown stdout", "teardown stderr"} {
						if !bytes.Contains(data, []byte(marker)) {
							t.Errorf("missing %q from gate log: %s", marker, data)
						}
					}
					found = true
				}
			}
			if !found {
				t.Fatal("custom stdout and stderr missing from logs")
			}
			t.Setenv("FAIL_GATE", "yes")
			stdout.Reset()
			stderr.Reset()
			code = run([]string{"test", "all", "--unit-only", flag}, strings.NewReader(""), &stdout, &stderr)
			if code != 7 || strings.Contains(stdout.String(), "custom stdout") || strings.Contains(stderr.String(), "custom stderr") {
				t.Fatalf("failed gate exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			var summary struct {
				Failed []struct {
					Log string `json:"log"`
				} `json:"failed"`
			}
			if flag == "--json-stream" {
				lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
				var event struct {
					Summary json.RawMessage `json:"summary"`
				}
				if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(event.Summary, &summary); err != nil {
					t.Fatal(err)
				}
			} else if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
				t.Fatal(err)
			}
			if len(summary.Failed) != 1 || summary.Failed[0].Log == "" {
				t.Fatalf("missing failure log: %#v", summary)
			}
			data, err := os.ReadFile(filepath.Join(root, summary.Failed[0].Log))
			if err != nil || !bytes.Contains(data, []byte("teardown stderr")) {
				t.Fatalf("failure log=%q err=%v", data, err)
			}
			t.Setenv("FAIL_GATE", "timeout")
			stdout.Reset()
			stderr.Reset()
			code = run([]string{"test", "all", "--unit-only", flag}, strings.NewReader(""), &stdout, &stderr)
			if code != 124 || strings.Contains(stdout.String(), "custom stdout") || strings.Contains(stderr.String(), "custom stderr") {
				t.Fatalf("timeout exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}
