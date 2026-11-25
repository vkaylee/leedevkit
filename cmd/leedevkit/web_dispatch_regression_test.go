package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/report"
)

func TestWebAndAllTargetsDispatchConfiguredServicesThroughAllPhases(t *testing.T) {
	for _, tc := range []struct {
		name     string
		services []string
	}{
		{name: "web", services: []string{"webdashboard"}},
		{name: "all", services: []string{"api", "webdashboard"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _, logPath := webDispatchFixture(t)
			t.Chdir(root)
			t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
			t.Setenv("COMPOSE_PROJECT_NAME", "dispatch-fixture")
			t.Setenv("COMPOSE_LOG", logPath)
			var stdout, stderr bytes.Buffer
			code := run([]string{"test", tc.name, "--json"}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%q", code, stderr.String())
			}
			var summary report.Summary
			if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
				t.Fatalf("decode summary %q: %v", stdout.String(), err)
			}
			if len(summary.Phases) != 3 {
				t.Fatalf("phases=%#v; want unit, integration, lint", summary.Phases)
			}
			for _, phase := range summary.Phases {
				if phase.Status != "passed" {
					t.Errorf("phase %q status=%q; want passed", phase.Phase, phase.Status)
				}
			}
			commands, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, service := range tc.services {
				if !strings.Contains(string(commands), " "+service+" bun ") && !strings.Contains(string(commands), " "+service+" go ") && !strings.Contains(string(commands), " "+service+" gofmt ") {
					t.Errorf("compose never executed service %q; commands=%q", service, commands)
				}
			}
		})
	}
}

func TestWebDispatchDoesNotReportFailedComposePhaseAsPassed(t *testing.T) {
	root, _, logPath := webDispatchFixture(t)
	t.Chdir(root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("COMPOSE_PROJECT_NAME", "dispatch-fixture")
	t.Setenv("COMPOSE_LOG", logPath)
	t.Setenv("COMPOSE_FAIL_SERVICE", "webdashboard")
	var stdout, stderr bytes.Buffer
	code := run([]string{"test", "web", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("failed compose phase returned success; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), `"status":"passed"`) || strings.Contains(stdout.String(), `"status": "passed"`) {
		t.Fatalf("failed compose phase reported pass: stdout=%q", stdout.String())
	}
}

func webDispatchFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	compose := filepath.Join(root, "fake-compose")
	logPath := filepath.Join(root, "compose.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$COMPOSE_LOG"
for arg in "$@"; do
  if [ "$arg" = "ps" ]; then
    printf '%s\n' '{"Service":"api","State":"running","Health":"healthy"}' '{"Service":"webdashboard","State":"running","Health":"healthy"}'
    exit 0
  fi
  if [ "$arg" = "exec" ]; then
    [ -z "$COMPOSE_FAIL_SERVICE" ] || ! printf '%s\n' "$*" | grep -q "exec .* $COMPOSE_FAIL_SERVICE " || exit 19
    exit 0
  fi
done
exit 0
`
	if err := os.WriteFile(compose, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	configuration := fmt.Sprintf(`
[container]
provider = 'fixture'
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ['compose', 'config', 'up', 'down', 'ps', 'build']
[services.api]
lang = 'go'
[services.webdashboard]
lang = 'typescript'
[targets]
web = ['webdashboard']
all = ['api', 'webdashboard']
[test.dependencies]
unit-api = ['api']
lint-api = ['api']
int-api = ['api']
unit-web = ['webdashboard']
lint-web = ['webdashboard']
e2e-web = ['webdashboard']
`, compose, compose)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".compose"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, compose, logPath
}
