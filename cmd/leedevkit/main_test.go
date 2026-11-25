package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/provider"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: leedevkit <command> [args...]") {
		t.Fatalf("help output missing usage: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("help stderr = %q", stderr.String())
	}
}

func TestRunTestHelpAliases(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("invalid = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, flag := range []string{"--help", "-h"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{"test", flag}, strings.NewReader(""), &stdout, &stderr); code != 0 {
				t.Fatalf("test help exit code = %d, stderr = %q", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage: leedevkit test") {
				t.Fatalf("test help output missing usage: %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("test help stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunNoArgsShowsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("no-args exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: leedevkit <command> [args...]") {
		t.Fatalf("no-args output missing usage: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("no-args stderr = %q", stderr.String())
	}
}

func TestRunVersionAliases(t *testing.T) {
	version = "0.10.0-test"
	t.Cleanup(func() { version = "dev" })
	for _, alias := range []string{"--version", "-v"} {
		t.Run(alias, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{alias}, strings.NewReader(""), &stdout, &stderr); code != 0 {
				t.Fatalf("version exit code = %d", code)
			}
			if got := strings.TrimSpace(stdout.String()); got != version {
				t.Fatalf("version = %q", got)
			}
			if stderr.Len() != 0 {
				t.Fatalf("version stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunUpdateRejectsExtraArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"update", "v1.2.3", "unexpected"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("update exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: leedevkit update [version]") {
		t.Fatalf("missing update usage: %q", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	version = "0.10.0-test"
	t.Cleanup(func() { version = "dev" })
	if code := run([]string{"version"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("version exit code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "0.10.0-test" {
		t.Fatalf("version = %q", got)
	}
}

func TestRunDoctorUsesGoReadOnlyPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[project]\nname = \"test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent", "rules"), 0o700); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	var stdout, stderr bytes.Buffer
	if code := run([]string{"doctor"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("doctor exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Project config:") {
		t.Fatalf("doctor output = %q", stderr.String())
	}
}

func TestParseDebugMCPArgs(t *testing.T) {
	options, err := parseDebugMCPArgs([]string{"debug", "mcp", "--environment", "fixture", "--manifest", "manifest.toml"})
	if err != nil || options.Environment != "fixture" || options.Manifest != "manifest.toml" {
		t.Fatalf("options = %#v, err = %v", options, err)
	}
}

func TestParseDebugMCPArgsRequiresManifestAndEnvironment(t *testing.T) {
	if _, err := parseDebugMCPArgs([]string{"debug", "mcp"}); err == nil {
		t.Fatal("missing debug arguments accepted")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"wat"}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("unknown command exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "Unknown command: wat") {
		t.Fatalf("unknown command stderr = %q", stderr.String())
	}
}

func TestResolveBaseRefValidatesWithoutShell(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"-c", "user.name=LeeDevKit Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	if got, err := resolveBaseRef(root, "HEAD"); err != nil || len(got) != 40 {
		t.Fatalf("resolved base ref = %q, err = %v", got, err)
	}
	if _, err := resolveBaseRef(root, "not-a-real-ref"); err == nil {
		t.Fatal("invalid base ref accepted")
	}
}

func TestConfiguredGoServiceNameUsesManifestService(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{
		"backend": {Language: "go", Name: "backend"},
	}}
	if got := configuredServiceName(project, "go"); got != "backend" {
		t.Fatalf("service = %q, want backend", got)
	}
}

func TestConfiguredGoServiceNameFallsBackToGo(t *testing.T) {
	if got := configuredServiceName(config.Project{}, "go"); got != "go" {
		t.Fatalf("service = %q, want go", got)
	}
}

func TestConfiguredGoServiceNamePrefersCanonicalGoServiceDeterministically(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{
		"apiserver": {Language: "go", Name: "apiserver"},
		"api":       {Language: "go", Name: "api"},
		"go":        {Language: "go", Name: "go"},
	}}
	if got := configuredServiceName(project, "go"); got != "go" {
		t.Fatalf("service = %q, want go", got)
	}
}

func TestRuntimeAssetRootUsesExecutableLocation(t *testing.T) {
	root := t.TempDir()
	asset := filepath.Join(root, "container", "go", "docker-compose.test.yml")
	if err := os.MkdirAll(filepath.Dir(asset), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(asset, []byte("services: {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := assetRootFromExecutable(filepath.Join(root, "bin", "leedevkit")); err != nil || got != root {
		t.Fatalf("asset root = %q, want %q; err=%v", got, root, err)
	}
	if _, err := assetRootFromExecutable(filepath.Join(t.TempDir(), "bin", "leedevkit")); err == nil {
		t.Fatal("missing global asset root accepted")
	}
}

func TestBuildTestLifecyclePlanMatchesProjectComposeContract(t *testing.T) {
	root := t.TempDir()
	composeFile := filepath.Join(root, ".compose", "docker-compose.test.yml")
	if err := os.MkdirAll(filepath.Dir(composeFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectConfig := config.Project{
		Services: map[string]config.Service{
			"apiserver": {Language: "go", Name: "apiserver"},
			"go":        {Language: "go", Name: "go"},
			"api":       {Language: "go", Name: "api"},
		},
		Dependencies: map[string][]string{"int-api": {"apiserver"}},
	}
	plan, service, err := buildTestLifecyclePlan(root, "leedevkit-test", "api", testProviderForMain(), projectConfig, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if service != "go" {
		t.Fatalf("service = %q, want go", service)
	}
	want := []string{"docker", "compose", "-p", "leedevkit-test", "-f", composeFile, "--profile", "go", "up", "-d", "--no-build", "go", "apiserver"}
	got, err := plan.Up()
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(got, want) {
		t.Fatalf("lifecycle args = %#v, want %#v", got, want)
	}
	defaultPlan, _, err := buildTestLifecyclePlan(root, "leedevkit-test", "api", testProviderForMain(), projectConfig, false, false)
	if err != nil {
		t.Fatal(err)
	}
	wantDefault := []string{"docker", "compose", "-p", "leedevkit-test", "-f", composeFile, "--profile", "go", "up", "-d", "go", "apiserver"}
	gotDefault, err := defaultPlan.Up()
	if err != nil || !sameStrings(gotDefault, wantDefault) {
		t.Fatalf("default lifecycle args = %#v, err = %v, want %#v", gotDefault, err, wantDefault)
	}
	buildPlan, _, err := buildTestLifecyclePlan(root, "leedevkit-test", "api", testProviderForMain(), projectConfig, true, false)
	if err != nil {
		t.Fatal(err)
	}
	wantBuild := []string{"docker", "compose", "-p", "leedevkit-test", "-f", composeFile, "--profile", "go", "up", "-d", "--build", "go", "apiserver"}
	gotBuild, err := buildPlan.Up()
	if err != nil || !sameStrings(gotBuild, wantBuild) {
		t.Fatalf("build lifecycle args = %#v, err = %v, want %#v", gotBuild, err, wantBuild)
	}
}

func testProviderForMain() provider.Provider {
	return provider.Provider{
		Name:         "docker",
		Compose:      []string{"docker", "compose"},
		Capabilities: []string{"compose", "config", "down", "ps", "up"},
	}
}

func TestBuildDatabaseQueryArgs(t *testing.T) {
	got, err := buildDatabaseQueryArgs(provider.Provider{CLI: []string{"docker"}}, "leedevkit-dev", "SELECT 1", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "exec", "-t", "leedevkit-dev-db", "psql", "-U", "lee", "-d", "leedevkit", "-c", "SELECT 1"}
	if !sameStrings(got, want) {
		t.Fatalf("query args = %#v, want %#v", got, want)
	}
}

func TestBuildDatabaseQueryJSONArgs(t *testing.T) {
	got, err := buildDatabaseQueryArgs(provider.Provider{CLI: []string{"docker"}}, "leedevkit-dev", "SELECT 1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(got[len(got)-3:], []string{"-A", "-t", "--json"}) {
		t.Fatalf("query JSON args = %#v", got)
	}
}

func TestParseDatabaseQueryArgs(t *testing.T) {
	sql, asJSON, err := parseDatabaseQueryArgs([]string{"db:query", "SELECT 1", "--json"})
	if err != nil || sql != "SELECT 1" || !asJSON {
		t.Fatalf("parsed query = %q, json = %v, err = %v", sql, asJSON, err)
	}
}

func TestParseDatabaseQueryRejectsMissingSQL(t *testing.T) {
	if _, _, err := parseDatabaseQueryArgs([]string{"db:query", "--json"}); err == nil {
		t.Fatal("missing SQL accepted")
	}
}

func TestParseMigrationCommand(t *testing.T) {
	for _, test := range []struct {
		args   []string
		action string
	}{
		{[]string{"migrate:run"}, "run"},
		{[]string{"migrate:revert"}, "revert"},
		{[]string{"migrate:status"}, "status"},
	} {
		t.Run(test.action, func(t *testing.T) {
			if got, err := parseMigrationCommand(test.args); err != nil || got != test.action {
				t.Fatalf("action = %q, err = %v", got, err)
			}
		})
	}
}

func TestParseMigrationCommandRejectsArguments(t *testing.T) {
	if _, err := parseMigrationCommand([]string{"migrate:run", "extra"}); err == nil {
		t.Fatal("migration arguments accepted")
	}
}

func TestBuildAPISyncCommandUsesTwoComposeExecs(t *testing.T) {
	got, err := buildAPISyncCommands([]string{"docker", "compose"}, "apiserver", "webdashboard")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"docker", "compose", "exec", "-T", "apiserver", "bash", "-c", "cargo run -- gen-openapi > /workspace/shared/openapi.json"},
		{"docker", "compose", "exec", "-T", "webdashboard", "bash", "-c", "npx openapi-typescript /workspace/shared/openapi.json -o ./src/types/api-generated.ts"},
	}
	if len(got) != len(want) || !sameStrings(got[0], want[0]) || !sameStrings(got[1], want[1]) {
		t.Fatalf("sync args = %#v, want %#v", got, want)
	}
}

func TestBuildGeneratedGateCommandUsesComposeExec(t *testing.T) {
	got, err := buildGeneratedGateCommand([]string{"compose"}, "go", "go", []string{"go", "generate", "./..."}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "exec", "-T", "go", "-w", "/workspace", "go", "generate", "./..."}
	if !sameStrings(got, want) {
		t.Fatalf("generated args = %#v, want %#v", got, want)
	}
}

func TestBuildGoCoverageCommandUsesProfileAndReport(t *testing.T) {
	got, err := buildGoCoverageCommand([]string{"compose"}, "go", "", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "exec", "-T", "go", "-w", "/workspace", "sh", "-c", "mkdir -p /workspace/.test_logs && go test -coverprofile=/workspace/.test_logs/coverage-go.out ./... && go tool cover -func=/workspace/.test_logs/coverage-go.out"}
	if !sameStrings(got, want) {
		t.Fatalf("coverage args = %#v, want %#v", got, want)
	}
}

func TestBuildGoCoverageCommandAddsPattern(t *testing.T) {
	got, err := buildGoCoverageCommand([]string{"compose"}, "backend", "auth.*", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || got[len(got)-1] != "-run=auth.*" || !strings.Contains(got[len(got)-3], `go test "$@"`) {
		t.Fatalf("coverage pattern missing: %#v", got)
	}
}

func TestBuildGeneratedGateCommandUsesConfiguredWorkdir(t *testing.T) {
	got, err := buildGeneratedGateCommand([]string{"compose"}, "backend", "backend", []string{"make", "generate"}, "services/backend")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got, "/workspace/services/backend") {
		t.Fatalf("generated workdir missing: %#v", got)
	}
}

func TestBuildAPISyncCommandRejectsMissingServices(t *testing.T) {
	if _, err := buildAPISyncCommands([]string{"compose"}, "", "web"); err == nil {
		t.Fatal("missing API service accepted")
	}
}

func TestBuildMigrationCommandUsesConfiguredRustService(t *testing.T) {
	got, err := buildMigrationCommand(
		"/repo",
		provider.Provider{Compose: []string{"docker", "compose"}},
		"leedevkit-test",
		"api",
		"run",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(got, []string{
		"docker", "compose", "-p", "leedevkit-test", "-f", "/repo/.compose/docker-compose.test.yml",
		"run", "-T", "--rm", "--workdir", "/workspace", "--entrypoint", "diesel", "api", "migration", "run",
	}) {
		t.Fatalf("migration args = %#v", got)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestPlanRunProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"plan":{"compose":["docker","compose"],"project":"test","files":["compose.yml"],"tool":"go","args":["version"],"service":"go"}}`
	if code := run([]string{"--plan-run"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("plan run exit code = %d, stderr = %q", code, stderr.String())
	}
	var response runProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.SchemaVersion != 1 || !contains(response.Args, "--entrypoint") || !contains(response.Args, "version") {
		t.Fatalf("run response = %#v", response)
	}
}

func TestPlanRunRejectsSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--plan-run"}, strings.NewReader(`{"schema_version":2}`), &stdout, &stderr); code != 2 {
		t.Fatalf("invalid run schema exit code = %d", code)
	}
}

func TestPlanTestProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"flags":{"target":"go","unit_only":true}}`
	if code := run([]string{"--plan-test"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("plan test exit code = %d, stderr = %q", code, stderr.String())
	}
	var response testProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.SchemaVersion != 1 || response.Result.Status != "blocked" {
		t.Fatalf("response = %#v", response)
	}
}

func TestResolveModesProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"services":[{"name":"backend","language":"go"},{"name":"frontend","language":"typescript"}]}`
	if code := run([]string{"--resolve-modes"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("resolve modes exit code = %d, stderr = %q", code, stderr.String())
	}
	var response modeProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("mode output is not JSON: %v", err)
	}
	if response.SchemaVersion != 1 || response.Modes["backend"] != "go" || response.Modes["frontend"] != "web" {
		t.Fatalf("mode response = %#v", response)
	}
}

func TestResolveTestTargetsProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"target":"all","all_services":["zkteco","api","standalone"],"aliases":[{"name":"go","services":["zkteco"]},{"name":"api","services":["api"]}]}`
	if code := run([]string{"--resolve-test-targets"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("resolve test targets exit code = %d, stderr = %q", code, stderr.String())
	}
	var response targetProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("test target output is not JSON: %v", err)
	}
	if response.SchemaVersion != 1 || !sameStrings(response.Targets, []string{"go", "api", "standalone"}) {
		t.Fatalf("test target response = %#v", response)
	}
}

func TestResolveTestTargetsRejectsSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--resolve-test-targets"}, strings.NewReader(`{"schema_version":2}`), &stdout, &stderr); code != 2 {
		t.Fatalf("invalid test target schema exit code = %d", code)
	}
}

func TestResolveTestTargetsRejectsMalformedJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--resolve-test-targets"}, strings.NewReader("{"), &stdout, &stderr); code != 2 {
		t.Fatalf("malformed test target exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "malformed test target request") {
		t.Fatalf("malformed test target stderr = %q", stderr.String())
	}
}

func TestResolveProviderProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"provider":{"provider":"docker","cli":["docker"],"compose":["docker","compose"],"capabilities":["compose"],"settings":{"password":"secret"},"source":"test"}}`
	if code := run([]string{"--resolve-provider"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("resolve provider exit code = %d, stderr = %q", code, stderr.String())
	}
	var response providerProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("provider output is not JSON: %v", err)
	}
	if response.SchemaVersion != 1 || response.Diagnostics["provider"] != "docker" {
		t.Fatalf("provider response = %#v", response)
	}
	settings, ok := response.Diagnostics["settings"].(map[string]any)
	if !ok || settings["password"] != "<redacted>" {
		t.Fatalf("provider settings = %#v", response.Diagnostics["settings"])
	}
}

func TestResolveProviderRejectsSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--resolve-provider"}, strings.NewReader(`{"schema_version":2}`), &stdout, &stderr); code != 2 {
		t.Fatalf("invalid provider schema exit code = %d", code)
	}
}

func TestPlanLifecycleProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"plan":{"provider":{"name":"docker","compose":["docker","compose"],"capabilities":["compose","config","up","down","ps","wait"]},"project":"test","profiles":["--profile","go"],"services":["go"],"dependencies":["postgres"],"build":false,"native_wait":true,"wait_timeout":120,"owns_project":true}}`
	if code := run([]string{"--plan-lifecycle"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("lifecycle exit code = %d, stderr = %q", code, stderr.String())
	}
	var response lifecycleProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("lifecycle output is not JSON: %v", err)
	}
	if response.SchemaVersion != 1 || len(response.Args) == 0 || response.Args[len(response.Args)-1] != "postgres" {
		t.Fatalf("lifecycle response = %#v", response)
	}
}

func TestPlanLifecycleRejectsSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--plan-lifecycle"}, strings.NewReader(`{"schema_version":2}`), &stdout, &stderr); code != 2 {
		t.Fatalf("invalid lifecycle schema exit code = %d", code)
	}
}

func TestProtocolRejectsTrailingJSON(t *testing.T) {
	for _, command := range []string{"--plan-lifecycle", "--resolve-modes", "--resolve-test-targets", "--resolve-provider", "--plan-gates"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			input := `{"schema_version":1} {"schema_version":1}`
			if code := run([]string{command}, strings.NewReader(input), &stdout, &stderr); code != 2 {
				t.Fatalf("trailing JSON exit code = %d, stderr = %q", code, stderr.String())
			}
		})
	}
}

func TestPlanLifecycleDownUsesOwnedTeardown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"operation":"down","plan":{"provider":{"name":"docker","compose":["docker","compose"],"capabilities":["compose","config","up","down","ps"]},"project":"test","owns_project":true}}`
	if code := run([]string{"--plan-lifecycle"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("lifecycle down exit code = %d, stderr = %q", code, stderr.String())
	}
	var response lifecycleProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("lifecycle output is not JSON: %v", err)
	}
	if response.Operation != "down" || !contains(response.Args, "--remove-orphans") || contains(response.Args, "--volumes") {
		t.Fatalf("lifecycle down response = %#v", response)
	}
}

func TestPlanGatesProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"gates":[{"name":"go-vet","phase":"lint","command":["go","vet","./..."]}]}`
	if code := run([]string{"--plan-gates"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("plan exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema_version":1`) || !strings.Contains(stdout.String(), "go-vet") {
		t.Fatalf("plan output = %q", stdout.String())
	}
}

func TestPlanGatesRejectsSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--plan-gates"}, strings.NewReader(`{"schema_version":2}`), &stdout, &stderr); code != 2 {
		t.Fatalf("plan invalid schema exit code = %d", code)
	}
}

func TestPlanGatesSelectsAndNormalizesDefinitions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"names":["custom","custom"],"definitions":{"custom":{"name":"custom","phase":"lint","command":["tool"]}}}`
	if code := run([]string{"--plan-gates"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("plan exit code = %d, stderr = %q", code, stderr.String())
	}
	var response gateProtocolResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("plan output is not JSON: %v", err)
	}
	if len(response.Gates) != 1 || response.Gates[0].Mode != "run" || response.Gates[0].Condition != "always" {
		t.Fatalf("normalized gates = %#v", response.Gates)
	}
}

func TestPlanGatesRejectsUnsafeWorkingDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"names":["custom"],"definitions":{"custom":{"name":"custom","phase":"lint","command":["tool"],"working_dir":"../outside"}}}`
	if code := run([]string{"--plan-gates"}, strings.NewReader(input), &stdout, &stderr); code != 2 {
		t.Fatalf("unsafe working dir exit code = %d", code)
	}
}

func TestPlanGatesRejectsMalformedJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--plan-gates"}, strings.NewReader("{"), &stdout, &stderr); code != 2 {
		t.Fatalf("malformed plan exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "malformed gate request") {
		t.Fatalf("malformed plan stderr = %q", stderr.String())
	}
}

func TestRunRejectsUnsupportedTestWithoutFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[project]\nname=\"test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	var stdout, stderr bytes.Buffer
	if code := run([]string{"test", "mixed"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("unsupported test exit code = %d, stderr = %q", code, stderr.String())
	}
	if strings.Contains(strings.ToLower(stderr.String()), "fallback") {
		t.Fatalf("test unexpectedly used fallback: %q", stderr.String())
	}
}

func TestGlobalComposeAssetsConfigFromUnrelatedCWDWithSpacedProjectRoot(t *testing.T) {
	compose, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker CLI unavailable")
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(sourceFile), "../.."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	projectRoot := filepath.Join(fixture, "project root")
	cwd := filepath.Join(fixture, "unrelated cwd")
	for _, path := range []string{projectRoot, cwd} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	type composeServiceConfig struct {
		Image string `json:"image"`
		Build struct {
			Context string `json:"context"`
		} `json:"build"`
		Healthcheck struct {
			Interval string `json:"interval"`
		} `json:"healthcheck"`
		Volumes []struct {
			Source string `json:"source"`
		} `json:"volumes"`
	}
	cases := []struct {
		asset       string
		profile     string
		service     string
		imagePrefix string
	}{
		{"go", "go", "go", "leedevkit-go:"},
		{"go", "infra", "infra", "leedevkit-go:"},
		{"rust", "rust", "rust", "leedevkit-rust:"},
	}
	for _, tc := range cases {
		composeFile := filepath.Join(repoRoot, "container", tc.asset, "docker-compose.test.yml")
		command := exec.Command(compose, "compose", "--profile", tc.profile, "-f", composeFile, "config", "--format", "json")
		command.Dir = cwd
		command.Env = append(os.Environ(), "LEEDEVKIT_PROJECT_ROOT="+projectRoot)
		output, err := command.Output()
		if err != nil {
			t.Fatalf("docker compose config for %s/%s: %v", tc.asset, tc.profile, err)
		}
		var parsed struct {
			Services map[string]composeServiceConfig `json:"services"`
		}
		if err := json.Unmarshal(output, &parsed); err != nil {
			t.Fatalf("parse compose config JSON for %s/%s: %v", tc.asset, tc.profile, err)
		}
		svc, ok := parsed.Services[tc.service]
		if !ok {
			t.Fatalf("service %q missing in %s/%s config", tc.service, tc.asset, tc.profile)
		}
		if !strings.HasPrefix(svc.Image, tc.imagePrefix) {
			t.Errorf("%s/%s image = %q, want prefix %q", tc.asset, tc.profile, svc.Image, tc.imagePrefix)
		}
		if svc.Healthcheck.Interval != "1s" {
			t.Errorf("%s/%s healthcheck interval = %q, want 1s", tc.asset, tc.profile, svc.Healthcheck.Interval)
		}
		wantContext := filepath.Join(repoRoot, "container", tc.asset)
		if filepath.Clean(svc.Build.Context) != filepath.Clean(wantContext) {
			t.Errorf("%s/%s build context = %q, want %q", tc.asset, tc.profile, svc.Build.Context, wantContext)
		}
		mounted := false
		for _, volume := range svc.Volumes {
			if filepath.Clean(volume.Source) == filepath.Clean(projectRoot) {
				mounted = true
				break
			}
		}
		if !mounted {
			t.Errorf("%s/%s volumes missing project mount %q: %#v", tc.asset, tc.profile, projectRoot, svc.Volumes)
		}
	}
}
