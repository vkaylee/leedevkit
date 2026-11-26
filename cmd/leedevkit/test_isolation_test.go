package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/vkaylee/leedevkit/internal/ownership"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
	"github.com/vkaylee/leedevkit/internal/report"
)

func writeGoIntegrationFixture(t *testing.T, root, gateTOML, configJSON string) (string, string, string) {
	t.Helper()
	compose := filepath.Join(root, "fake-compose")
	cmdLog := filepath.Join(root, "compose.log")
	execLog := filepath.Join(root, "exec.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$COMPOSE_LOG"
for arg in "$@"; do
  if [ "$arg" = "config" ]; then
    if [ -n "${FAKE_CONFIG_JSON:-}" ]; then
      printf '%s\n' "$FAKE_CONFIG_JSON"
      exit 0
    fi
    exit 1
  fi
  if [ "$arg" = "ps" ]; then
    printf '%s\n' '{"Service":"apiserver","State":"running","Health":"healthy"}'
    exit 0
  fi
  if [ "$arg" = "exec" ]; then
    printf 'ENV:%s|%s|ARGS:%s\n' "${APP_DB_INTEGRATION:-}" "${HOST_SECRET_TOKEN:-}" "$*" >> "$EXEC_LOG"
    if [ "${FAIL_MIGRATE:-}" = "1" ] && printf '%s\n' "$*" | grep -q "cmd/migrate"; then
      printf 'migration error: dial postgres://app:topsecretpass@postgres:5432/testdb failed\n' >&2
      exit 4
    fi
    if [ "${REQUIRE_INT_ENV:-}" = "1" ] && printf '%s\n' "$*" | grep -q -- "-tags=integration"; then
      if [ "${APP_DB_INTEGRATION:-}" != "1" ]; then
        printf 'integration env missing\n' >&2
        exit 5
      fi
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
	manifest := fmt.Sprintf(`
[project]
name = "SampleApp"
namespace = "sampleapp"
languages = ["go"]

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

[targets]
api = ["apiserver"]
all = ["apiserver"]

[test.dependencies]
unit-api = ["apiserver"]
int-api = ["postgres", "apiserver"]
lint-api = ["apiserver"]

%s
`, compose, compose, gateTOML)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if configJSON != "" {
		t.Setenv("FAKE_CONFIG_JSON", configJSON)
	}
	t.Setenv("COMPOSE_LOG", cmdLog)
	t.Setenv("EXEC_LOG", execLog)
	return compose, cmdLog, execLog
}

func TestGoIntegrationEnvPropagationSetupAndSecretHygiene(t *testing.T) {
	root := t.TempDir()
	gateTOML := `
[test.gates.int-api]
env = { APP_DB_INTEGRATION = "1" }
environment = ["HOST_SECRET_TOKEN"]
required_env = ["APP_DB_INTEGRATION", "DATABASE_URL"]
setup_command = ["go", "run", "./cmd/migrate", "up"]
`
	configJSON := `{"services":{"apiserver":{"environment":{"DATABASE_URL":"postgres://test:test@postgres:5432/testdb"}}},"volumes":{}}`
	_, cmdLog, execLog := writeGoIntegrationFixture(t, root, gateTOML, configJSON)

	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("HOST_SECRET_TOKEN", "s3cr3t-value-never-print")
	t.Setenv("REQUIRE_INT_ENV", "1")

	var stdout, stderr bytes.Buffer
	code := run([]string{"test", "api", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var summary report.Summary
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal: %v (%q)", err, stdout.String())
	}
	if summary.Status != "passed" || !summary.FullRegression || summary.Counts.Passed != 3 || summary.Counts.Failed != 0 || summary.Counts.Skipped != 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	// Verify secret values never leaked into stdout, stderr, or compose argv.
	cmdBytes, err := os.ReadFile(cmdLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, stream := range []struct {
		name string
		text string
	}{
		{"stdout", stdout.String()},
		{"stderr", stderr.String()},
		{"compose argv", string(cmdBytes)},
	} {
		if strings.Contains(stream.text, "s3cr3t-value-never-print") || strings.Contains(stream.text, "APP_DB_INTEGRATION=1") {
			t.Fatalf("%s leaked env/secret value:\n%s", stream.name, stream.text)
		}
	}
	execBytes, err := os.ReadFile(execLog)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(execBytes)), "\n")
	migrateIdx, intIdx, unitIdx := -1, -1, -1
	for i, line := range lines {
		switch {
		case strings.Contains(line, "cmd/migrate"):
			migrateIdx = i
			if !strings.HasPrefix(line, "ENV:1|s3cr3t-value-never-print|") {
				t.Fatalf("migrate did not receive configured env vars: %q", line)
			}
			if !strings.Contains(line, "-e APP_DB_INTEGRATION") || !strings.Contains(line, "-e HOST_SECRET_TOKEN") {
				t.Fatalf("migrate missing -e flags: %q", line)
			}
		case strings.Contains(line, "-tags=integration"):
			intIdx = i
			if !strings.HasPrefix(line, "ENV:1|s3cr3t-value-never-print|") {
				t.Fatalf("integration test did not receive configured env vars: %q", line)
			}
		case strings.Contains(line, "go test -json ./..."):
			unitIdx = i
			if strings.HasPrefix(line, "ENV:1|") {
				t.Fatalf("unit test must not receive integration-only env var: %q", line)
			}
		}
	}
	if unitIdx < 0 || migrateIdx < 0 || intIdx < 0 || migrateIdx >= intIdx {
		t.Fatalf("expected unit, then migrate before integration; unit=%d migrate=%d int=%d\n%s", unitIdx, migrateIdx, intIdx, execBytes)
	}
}

func TestGoIntegrationFailsClosedOnMissingDBConfigOrFailedMigration(t *testing.T) {
	t.Run("missing required_env DATABASE_URL blocks integration before setup or tests", func(t *testing.T) {
		root := t.TempDir()
		gateTOML := `
[test.gates.int-api]
env = { APP_DB_INTEGRATION = "1" }
required_env = ["APP_DB_INTEGRATION", "DATABASE_URL"]
setup_command = ["go", "run", "./cmd/migrate", "up"]
`
		// Compose service has NO DATABASE_URL defined and host has none.
		configJSON := `{"services":{"apiserver":{"environment":{}}},"volumes":{}}`
		_, _, execLog := writeGoIntegrationFixture(t, root, gateTOML, configJSON)

		t.Chdir(root)
		t.Setenv("HOME", root)
		t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
		t.Setenv("DATABASE_URL", "")

		var stdout, stderr bytes.Buffer
		code := run([]string{"test", "api", "--json"}, strings.NewReader(""), &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected non-zero exit when DATABASE_URL missing, got 0 stdout=%q", stdout.String())
		}
		var summary report.Summary
		if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
			t.Fatalf("unmarshal: %v (%q)", err, stdout.String())
		}
		if summary.Status != "blocked" || summary.FullRegression || summary.Counts.Passed != 2 || summary.Counts.Blocked != 1 {
			t.Fatalf("expected unit+lint pass and integration blocked, got %#v", summary)
		}
		if len(summary.Blocked) != 1 || !strings.Contains(summary.Blocked[0].Diagnostic, "DATABASE_URL") {
			t.Fatalf("expected diagnostic naming DATABASE_URL, got %#v", summary.Blocked)
		}
		execBytes, _ := os.ReadFile(execLog)
		if strings.Contains(string(execBytes), "cmd/migrate") || strings.Contains(string(execBytes), "-tags=integration") {
			t.Fatalf("setup/integration must not run when required_env is missing:\n%s", execBytes)
		}
	})

	t.Run("failing migrate setup_command fails integration, skips go test, and redacts DSN password", func(t *testing.T) {
		root := t.TempDir()
		gateTOML := `
[test.gates.int-api]
env = { APP_DB_INTEGRATION = "1" }
required_env = ["APP_DB_INTEGRATION", "DATABASE_URL"]
setup_command = ["go", "run", "./cmd/migrate", "up"]
`
		configJSON := `{"services":{"apiserver":{"environment":{"DATABASE_URL":"postgres://app:topsecretpass@postgres:5432/testdb"}}},"volumes":{}}`
		_, _, execLog := writeGoIntegrationFixture(t, root, gateTOML, configJSON)

		t.Chdir(root)
		t.Setenv("HOME", root)
		t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
		t.Setenv("FAIL_MIGRATE", "1")

		var stdout, stderr bytes.Buffer
		code := run([]string{"test", "api", "--json"}, strings.NewReader(""), &stdout, &stderr)
		if code != 4 {
			t.Fatalf("expected migrate exit code 4, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		var summary report.Summary
		if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
			t.Fatalf("unmarshal: %v (%q)", err, stdout.String())
		}
		if summary.Status != "failed" || summary.FullRegression || summary.Counts.Passed != 2 || summary.Counts.Failed != 1 {
			t.Fatalf("expected unit+lint pass and integration failed, got %#v", summary)
		}
		if strings.Contains(stdout.String(), "topsecretpass") || strings.Contains(stderr.String(), "topsecretpass") {
			t.Fatalf("DSN password leaked in output: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
		execBytes, _ := os.ReadFile(execLog)
		if strings.Contains(string(execBytes), "-tags=integration") {
			t.Fatalf("integration test must not run after migrate failure:\n%s", execBytes)
		}
	})

	t.Run("phase failure when unit and lint pass still exits non-zero and reports distinct statuses", func(t *testing.T) {
		root := t.TempDir()
		_, _, _ = writeGoIntegrationFixture(t, root, "", "")
		t.Chdir(root)
		t.Setenv("HOME", root)
		t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
		// Default integration gate does NOT set APP_DB_INTEGRATION=1 -> simulated integration test fails.
		t.Setenv("REQUIRE_INT_ENV", "1")

		var stdout, stderr bytes.Buffer
		code := run([]string{"test", "api", "--json"}, strings.NewReader(""), &stdout, &stderr)
		if code != 5 {
			t.Fatalf("expected integration failure exit 5, got %d stdout=%q", code, stdout.String())
		}
		var summary report.Summary
		if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if summary.Status != "failed" || summary.FullRegression || summary.ExitCode != 5 {
			t.Fatalf("expected failed summary exit 5, got %#v", summary)
		}
		phaseStatus := map[string]string{}
		for _, p := range summary.Phases {
			phaseStatus[p.Phase] = p.Status
		}
		if phaseStatus["Unit Tests"] != "passed" || phaseStatus["Integration Tests"] != "failed" || phaseStatus["Linting"] != "passed" {
			t.Fatalf("expected distinct phase statuses, got %#v", phaseStatus)
		}
	})

	t.Run("refuses dev Compose project and ignores COMPOSE_PROJECT_NAME hijack", func(t *testing.T) {
		root := t.TempDir()
		_, cmdLog, _ := writeGoIntegrationFixture(t, root, "", "")
		t.Chdir(root)
		t.Setenv("HOME", root)
		t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")

		t.Setenv("LEEDEVKIT_TEST_PROJECT", "sampleapp-dev")
		var stdout, stderr bytes.Buffer
		code := run([]string{"test", "api", "--json"}, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "refusing to run tests against dev Compose project") {
			t.Fatalf("expected dev project rejection exit 2, got code=%d stderr=%q", code, stderr.String())
		}
		if _, err := os.Stat(cmdLog); err == nil {
			t.Fatal("compose must not execute when dev project is rejected")
		}

		// With LEEDEVKIT_TEST_PROJECT unset and COMPOSE_PROJECT_NAME=sampleapp-dev, tests must use isolated test project.
		t.Setenv("LEEDEVKIT_TEST_PROJECT", "")
		t.Setenv("COMPOSE_PROJECT_NAME", "sampleapp-dev")
		stdout.Reset()
		stderr.Reset()
		code = run([]string{"test", "api", "--json"}, strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected isolated test run exit 0, got %d stderr=%q", code, stderr.String())
		}
		cmdBytes, err := os.ReadFile(cmdLog)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(cmdBytes), "-p sampleapp-dev") || !strings.Contains(string(cmdBytes), "-p sampleapp-test-") {
			t.Fatalf("COMPOSE_PROJECT_NAME hijacked test stack:\n%s", cmdBytes)
		}
	})
}

func TestConcurrentTestRunsIsolatePerRunAndRejectPinnedLockCollision(t *testing.T) {
	t.Run("two parallel runs get distinct isolated Compose projects and both pass", func(t *testing.T) {
		root := t.TempDir()
		compose := filepath.Join(root, "fake-compose")
		projectsFile := filepath.Join(root, "projects.log")
		barrierDir := filepath.Join(root, "barrier")
		if err := os.MkdirAll(barrierDir, 0o700); err != nil {
			t.Fatal(err)
		}
		script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "ps" ]; then
    printf '%%s\n' '{"Service":"apiserver","State":"running","Health":"healthy"}'
    exit 0
  fi
  if [ "$arg" = "up" ]; then
    printf '%%s\n' "${LEEDEVKIT_TEST_PROJECT:-}" >> %q
    touch %q/"${LEEDEVKIT_TEST_PROJECT:-unknown}"
    i=0
    while [ "$i" -lt 40 ]; do
      count=$(ls -1 %q | wc -l)
      if [ "$count" -ge 2 ]; then
        break
      fi
      sleep 0.02
      i=$((i + 1))
    done
    exit 0
  fi
done
exit 0
`, projectsFile, barrierDir, barrierDir)
		if err := os.WriteFile(compose, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, ".compose"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		manifest := fmt.Sprintf(`
[project]
namespace = "parallelapp"
[container]
provider = "fixture"
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ["compose", "config", "up", "down", "ps", "build"]
[services.apiserver]
lang = "go"
[targets]
api = ["apiserver"]
[test.dependencies]
unit-api = ["apiserver"]
`, compose, compose)
		if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		t.Setenv("HOME", root)
		t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")

		codes := make([]int, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				var out, errOut bytes.Buffer
				codes[idx] = run([]string{"test", "api", "--unit-only", "--json"}, strings.NewReader(""), &out, &errOut)
				if codes[idx] != 0 {
					t.Errorf("worker %d exit=%d stdout=%q stderr=%q", idx, codes[idx], out.String(), errOut.String())
				}
			}(i)
		}
		wg.Wait()
		entries, err := os.ReadDir(barrierDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 distinct isolated projects running concurrently, got %d", len(entries))
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), "parallelapp-test-") {
				t.Fatalf("unexpected project name %q", entry.Name())
			}
		}
	})

	t.Run("pinned LEEDEVKIT_TEST_PROJECT fails closed when already locked by another run", func(t *testing.T) {
		root := t.TempDir()
		_, cmdLog, _ := writeGoIntegrationFixture(t, root, "", "")
		t.Chdir(root)
		t.Setenv("HOME", root)
		t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")

		pinnedProject := "sampleapp-test-pinned-lock"
		holder, err := ownership.Open(pinnedProject, true)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close(nil)

		t.Setenv("LEEDEVKIT_TEST_PROJECT", pinnedProject)
		var stdout, stderr bytes.Buffer
		code := run([]string{"test", "api", "--unit-only", "--json"}, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "is locked by another active test run") {
			t.Fatalf("expected lock collision exit 2, got code=%d stderr=%q", code, stderr.String())
		}
		if _, err := os.Stat(cmdLog); err == nil {
			t.Fatal("compose must not run when pinned test project is locked")
		}
	})
}

func TestPrepareComposeTestRuntimeHandlesForeignVolumeOwnershipAndFixedVolumeLock(t *testing.T) {
	var cliCalls [][]string
	configJSON := `{
		"services": {"apiserver": {"environment": {"DATABASE_URL": "postgres://test:test@postgres:5432/test"}}},
		"volumes": {
			"pgdata": {"name": "myapp-test-1234_pgdata"},
			"shared_cache": {"name": "legacy_shared_cache", "labels": {"owner": "old"}},
			"ext_cache": {"name": "leedevkit_go-module-cache", "external": true}
		}
	}`
	runFake := func(_ context.Context, req process.Request) process.Result {
		cliCalls = append(cliCalls, append([]string(nil), req.Args...))
		joined := strings.Join(req.Args, " ")
		switch {
		case strings.Contains(joined, "config --format json"):
			return process.Result{Stdout: []byte(configJSON)}
		case strings.Contains(joined, "volume inspect"):
			return process.Result{Stdout: []byte("legacy-other-project\n")}
		default:
			return process.Result{}
		}
	}
	prov := provider.Provider{
		Name:         "fixture",
		CLI:          []string{"docker"},
		Compose:      []string{"docker", "compose"},
		Capabilities: []string{"compose", "config", "up", "down", "ps"},
	}

	rt, err := prepareComposeTestRuntime(context.Background(), prov, "myapp-test-1234", []string{".compose/docker-compose.test.yml"}, t.TempDir(), nil, runFake)
	if err != nil {
		t.Fatalf("prepareComposeTestRuntime failed: %v", err)
	}
	if rt.OverrideFile == "" {
		t.Fatal("expected external override file for foreign-owned fixed volume")
	}
	overrideBytes, err := os.ReadFile(rt.OverrideFile)
	if err != nil {
		t.Fatal(err)
	}
	overrideText := string(overrideBytes)
	if !strings.Contains(overrideText, `"shared_cache": !override`) || !strings.Contains(overrideText, `name: "legacy_shared_cache"`) || !strings.Contains(overrideText, "external: true") {
		t.Fatalf("unexpected volume override YAML:\n%s", overrideText)
	}
	if strings.Contains(overrideText, "pgdata") || strings.Contains(overrideText, "ext_cache") {
		t.Fatalf("override must only include foreign non-external fixed volumes:\n%s", overrideText)
	}
	for _, call := range cliCalls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "volume rm") || strings.Contains(joined, "volume prune") {
			t.Fatalf("volume ownership handling must never delete user volumes: %#v", call)
		}
	}
	// While rt holds the fixed volume lock, a second concurrent test run referencing the same volume must fail closed.
	if _, err := prepareComposeTestRuntime(context.Background(), prov, "myapp-test-5678", []string{".compose/docker-compose.test.yml"}, t.TempDir(), nil, runFake); err == nil || !strings.Contains(err.Error(), "legacy_shared_cache") {
		t.Fatalf("expected fixed volume lock rejection, got err=%v", err)
	}

	overridePath := rt.OverrideFile
	rt.Close()
	if _, err := os.Stat(overridePath); !os.IsNotExist(err) {
		t.Fatalf("expected temporary override file removed on Close(), stat err=%v", err)
	}
	// After Close(), another run can acquire the volume lock cleanly.
	rt2, err := prepareComposeTestRuntime(context.Background(), prov, "myapp-test-5678", []string{".compose/docker-compose.test.yml"}, t.TempDir(), nil, runFake)
	if err != nil {
		t.Fatalf("expected lock release after Close(), got err=%v", err)
	}
	rt2.Close()
}

func TestSingleTargetGoTestSupportsGateEnvIsolationAndRefusesDevProject(t *testing.T) {
	root := t.TempDir()
	compose := filepath.Join(root, "fake-compose")
	cmdLog := filepath.Join(root, "compose.log")
	execLog := filepath.Join(root, "exec.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$COMPOSE_LOG"
for arg in "$@"; do
  if [ "$arg" = "config" ]; then
    printf '{"services":{"go":{"environment":{"DATABASE_URL":"postgres://test:test@postgres:5432/test"}}},"volumes":{}}\n'
    exit 0
  fi
  if [ "$arg" = "ps" ]; then
    printf '[{"Service":"go","State":"running","Health":"healthy"}]\n'
    exit 0
  fi
  if [ "$arg" = "exec" ]; then
    printf 'ENV:%s|ARGS:%s\n' "${APP_DB_INTEGRATION:-}" "$*" >> "$EXEC_LOG"
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
	manifest := fmt.Sprintf(`
[project]
namespace = "soloapp"
[container]
provider = "fixture"
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ["compose", "config", "up", "down", "ps", "build"]
[services.go]
lang = "go"
[targets]
go = ["go"]
all = ["go"]
[test.dependencies]
int-go = ["postgres", "go"]
[test.gates.int-go]
env = { APP_DB_INTEGRATION = "1" }
required_env = ["APP_DB_INTEGRATION", "DATABASE_URL"]
setup_command = ["go", "run", "./cmd/migrate", "up"]
`, compose, compose)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("COMPOSE_LOG", cmdLog)
	t.Setenv("EXEC_LOG", execLog)

	// Reject dev Compose project.
	t.Setenv("LEEDEVKIT_TEST_PROJECT", "soloapp-dev")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"test", "go", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("expected exit 2 for dev project, got %d stderr=%q", code, stderr.String())
	}

	// Reject locked pinned project.
	holder, err := ownership.Open("soloapp-test-locked", true)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close(nil)
	t.Setenv("LEEDEVKIT_TEST_PROJECT", "soloapp-test-locked")
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"test", "go", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("expected exit 2 for locked pinned project, got %d stderr=%q", code, stderr.String())
	}

	// Normal isolated run ignores COMPOSE_PROJECT_NAME=soloapp-dev and runs migrate + integration with env.
	t.Setenv("LEEDEVKIT_TEST_PROJECT", "")
	t.Setenv("COMPOSE_PROJECT_NAME", "soloapp-dev")
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"test", "go", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("expected exit 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	execBytes, err := os.ReadFile(execLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(execBytes), "ENV:1|") || !strings.Contains(string(execBytes), "cmd/migrate") {
		t.Fatalf("expected setup_command and env propagation in single-target Go path:\n%s", execBytes)
	}
	cmdBytes, err := os.ReadFile(cmdLog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cmdBytes), "-p soloapp-dev") || !strings.Contains(string(cmdBytes), "-p soloapp-test-") {
		t.Fatalf("expected isolated soloapp-test-<runid> project:\n%s", cmdBytes)
	}
}
