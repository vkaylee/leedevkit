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

func writeFakePolyglotCompose(t *testing.T, root string) (string, string, string) {
	t.Helper()
	compose := filepath.Join(root, "fake-compose")
	logPath := filepath.Join(root, "compose.log")
	envLogPath := filepath.Join(root, "compose.env")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$COMPOSE_LOG"
printf 'TEST_PROJECT=%s|COMPOSE_PROJECT=%s|NAMESPACE=%s\n' "${LEEDEVKIT_TEST_PROJECT:-}" "${COMPOSE_PROJECT_NAME:-}" "${LEEDEVKIT_PROJECT_NAMESPACE:-}" >> "$COMPOSE_ENV_LOG"
for arg in "$@"; do
  if [ "$arg" = "ps" ]; then
    printf '%s\n' '{"Service":"apiserver","State":"running","Health":"healthy"}' '{"Service":"webdashboard","State":"running","Health":"healthy"}'
    exit 0
  fi
  if [ "$arg" = "exec" ]; then
    if [ "${FAKE_UNFORMATTED:-}" = "1" ] && printf '%s\n' "$*" | grep -q "gofmt -l"; then
      printf 'apiserver/main.go\n'
      exit 0
    fi
    if [ "${FAKE_FAIL_TYPECHECK:-}" = "1" ] && printf '%s\n' "$*" | grep -q "type-check"; then
      printf 'TS2322: Type string is not assignable to number\n' >&2
      exit 2
    fi
    exit 0
  fi
done
exit 0
`
	if err := os.WriteFile(compose, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".compose"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return compose, logPath, envLogPath
}

func TestIntAPIRunsDefaultGoIntegrationGateAndUnifiesNamespaceProject(t *testing.T) {
	root := t.TempDir()
	compose, logPath, envLogPath := writeFakePolyglotCompose(t, root)
	manifest := fmt.Sprintf(`
[project]
name = "LeeAttend"
namespace = "leeattend"
languages = ["go", "typescript"]

[container]
provider = "fixture"
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ["compose", "config", "up", "down", "ps", "build"]

[services.apiserver]
lang = "go"
go = true
workdir = "apiserver"

[services.webdashboard]
lang = "typescript"
dockerfile = "./webdashboard"

[targets]
api = ["apiserver"]
web = ["webdashboard"]
all = ["apiserver", "webdashboard"]

[test.dependencies]
unit-api = ["apiserver"]
int-api = ["postgres", "redis", "apiserver"]
lint-api = ["apiserver"]
unit-web = ["bun_init", "webdashboard"]
e2e-web = ["bun_init", "webdashboard"]
lint-web = ["bun_init", "webdashboard"]
`, compose, compose)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("COMPOSE_LOG", logPath)
	t.Setenv("COMPOSE_ENV_LOG", envLogPath)

	var stdout, stderr bytes.Buffer
	code := run([]string{"test", "all", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var summary report.Summary
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v (%q)", err, stdout.String())
	}
	if summary.Status != "passed" || !summary.FullRegression || summary.Counts.Skipped != 0 {
		t.Fatalf("expected passed full regression with 0 skips, got %#v", summary)
	}
	cmdsBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cmds := string(cmdsBytes)
	if !strings.Contains(cmds, "-p leeattend-test") || strings.Contains(cmds, "leedevkit-test-") {
		t.Fatalf("expected unified project name leeattend-test without hashed project:\n%s", cmds)
	}
	if !strings.Contains(cmds, "--profile int-api up -d apiserver postgres redis") {
		t.Fatalf("expected int-api disposable stack up with postgres+redis+apiserver:\n%s", cmds)
	}
	if !strings.Contains(cmds, "apiserver go test -json -tags=integration ./...") {
		t.Fatalf("expected default Go integration gate execution:\n%s", cmds)
	}
	envBytes, err := os.ReadFile(envLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envBytes), "TEST_PROJECT=leeattend-test|COMPOSE_PROJECT=leeattend-test|NAMESPACE=leeattend") {
		t.Fatalf("expected exported LEEDEVKIT_TEST_PROJECT & COMPOSE_PROJECT_NAME:\n%s", envBytes)
	}
}

func TestIntAPIFailsClosedWhenDeclaredDependencyHasNoResolvableGate(t *testing.T) {
	root := t.TempDir()
	compose, logPath, envLogPath := writeFakePolyglotCompose(t, root)
	// [quality.go] disables DefaultGoGates while omitting integration, yet [test.dependencies] declares int-api!
	manifest := fmt.Sprintf(`
[project]
namespace = "leeattend"
[container]
provider = "fixture"
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ["compose", "config", "up", "down", "ps", "build"]
[services.apiserver]
lang = "go"
go = true
[services.webdashboard]
lang = "typescript"
[targets]
api = ["apiserver"]
web = ["webdashboard"]
all = ["apiserver", "webdashboard"]
[quality.gates.u]
phase = "unit"
command = ["go", "test", "./..."]
required = true
[quality.gates.l]
phase = "lint"
command = ["go", "vet", "./..."]
required = true
[quality.go]
unit = ["u"]
lint = ["l"]
[test.dependencies]
unit-api = ["apiserver"]
int-api = ["apiserver"]
lint-api = ["apiserver"]
unit-web = ["webdashboard"]
e2e-web = ["webdashboard"]
lint-web = ["webdashboard"]
`, compose, compose)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("COMPOSE_LOG", logPath)
	t.Setenv("COMPOSE_ENV_LOG", envLogPath)

	for _, target := range []string{"all", "api"} {
		t.Run(target, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"test", target, "--json"}, strings.NewReader(""), &stdout, &stderr)
			if code == 0 {
				t.Fatalf("target %s exited 0 despite unresolvable int-api gate! stdout=%q", target, stdout.String())
			}
			var summary report.Summary
			if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
				t.Fatalf("unmarshal: %v (%q)", err, stdout.String())
			}
			if summary.Status == "passed" || summary.FullRegression || summary.Status != "blocked" {
				t.Fatalf("expected fail-closed blocked summary, got %#v", summary)
			}
		})
	}
}

func TestUnexpectedIntegrationSkipAndLintTypecheckFailureNeverReportOverallPass(t *testing.T) {
	t.Run("missing int-api dependency never exits zero or reports passed", func(t *testing.T) {
		root := t.TempDir()
		compose, logPath, envLogPath := writeFakePolyglotCompose(t, root)
		manifest := fmt.Sprintf(`
[project]
namespace = "polyglot"
[container]
provider = "fixture"
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ["compose", "config", "up", "down", "ps", "build"]
[services.apiserver]
lang = "go"
[services.webdashboard]
lang = "typescript"
[targets]
all = ["apiserver", "webdashboard"]
[test.dependencies]
unit-apiserver = ["apiserver"]
lint-apiserver = ["apiserver"]
unit-webdashboard = ["webdashboard"]
e2e-webdashboard = ["webdashboard"]
lint-webdashboard = ["webdashboard"]
`, compose, compose)
		if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		t.Setenv("HOME", root)
		t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
		t.Setenv("COMPOSE_LOG", logPath)
		t.Setenv("COMPOSE_ENV_LOG", envLogPath)

		var stdout, stderr bytes.Buffer
		code := run([]string{"test", "all", "--json"}, strings.NewReader(""), &stdout, &stderr)
		if code == 0 {
			t.Fatalf("unexpected integration skip returned exit 0! stdout=%q", stdout.String())
		}
		var summary report.Summary
		if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
			t.Fatal(err)
		}
		if summary.Status == "passed" || summary.FullRegression || len(summary.Failures) == 0 {
			t.Fatalf("unexpected skip must never pass: %#v", summary)
		}
	})

	t.Run("gofmt unformatted or web type-check failure fails test all", func(t *testing.T) {
		for _, envToggle := range []string{"FAKE_UNFORMATTED", "FAKE_FAIL_TYPECHECK"} {
			t.Run(envToggle, func(t *testing.T) {
				root := t.TempDir()
				compose, logPath, envLogPath := writeFakePolyglotCompose(t, root)
				manifest := fmt.Sprintf(`
[project]
namespace = "polyglot"
[container]
provider = "fixture"
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ["compose", "config", "up", "down", "ps", "build"]
[services.apiserver]
lang = "go"
[services.webdashboard]
lang = "typescript"
[targets]
all = ["apiserver", "webdashboard"]
[test.dependencies]
unit-apiserver = ["apiserver"]
int-apiserver = ["apiserver"]
lint-apiserver = ["apiserver"]
unit-webdashboard = ["webdashboard"]
e2e-webdashboard = ["webdashboard"]
lint-webdashboard = ["webdashboard"]
`, compose, compose)
				if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
					t.Fatal(err)
				}
				t.Chdir(root)
				t.Setenv("HOME", root)
				t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
				t.Setenv("COMPOSE_LOG", logPath)
				t.Setenv("COMPOSE_ENV_LOG", envLogPath)
				t.Setenv(envToggle, "1")

				var stdout, stderr bytes.Buffer
				code := run([]string{"test", "all", "--json"}, strings.NewReader(""), &stdout, &stderr)
				if code == 0 {
					t.Fatalf("%s returned exit 0! stdout=%q", envToggle, stdout.String())
				}
				var summary report.Summary
				if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
					t.Fatal(err)
				}
				if summary.Status != "failed" || summary.FullRegression {
					t.Fatalf("%s summary=%#v", envToggle, summary)
				}
			})
		}
	})
}
