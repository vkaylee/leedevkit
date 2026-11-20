package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"wat"}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("unknown command exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "Unknown command: wat") {
		t.Fatalf("unknown command stderr = %q", stderr.String())
	}
}

func TestInfraLintCommandsUseRuntimeScripts(t *testing.T) {
	runtime := filepath.Join("/runtime")
	project := filepath.Join("/project")
	commands := infraLintCommands(runtime, project)
	if len(commands) < 2 {
		t.Fatalf("command count = %d, want at least 2", len(commands))
	}
	want := [][]string{
		{filepath.Join(runtime, ".venv", "bin", "ruff"), "check", filepath.Join(runtime, "scripts")},
		{filepath.Join(runtime, ".venv", "bin", "mypy"), filepath.Join(runtime, "scripts")},
	}
	for index := range want {
		if !sameStrings(commands[index], want[index]) {
			t.Fatalf("command %d = %#v, want %#v", index, commands[index], want[index])
		}
	}
}

func TestInfraLintStopsOnFailure(t *testing.T) {
	runtime := t.TempDir()
	venvBin := filepath.Join(runtime, ".venv", "bin")
	if err := os.MkdirAll(venvBin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"ruff", "mypy"} {
		path := filepath.Join(venvBin, tool)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runInfraLint(runtime, t.TempDir(), strings.NewReader(""), &stdout, &stderr); code != 7 {
		t.Fatalf("lint exit code = %d, want 7", code)
	}
}

func TestInfraUnitCommandUsesPytestCoverage(t *testing.T) {
	runtime := filepath.Join("/runtime")
	command := infraUnitCommand(runtime, "")
	wantPrefix := []string{
		filepath.Join(runtime, ".venv", "bin", "pytest"),
		"--cov=scripts",
		"--cov-config",
		filepath.Join(runtime, "scripts", ".coveragerc"),
		"--cov-report=term-missing",
		"--cov-report=html:.test_logs/coverage-production",
		"--cov-fail-under=80",
	}
	if len(command) < len(wantPrefix) {
		t.Fatalf("unit command = %#v", command)
	}
	if !sameStrings(command[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("unit command prefix = %#v, want %#v", command[:len(wantPrefix)], wantPrefix)
	}
}

func TestInfraUnitStopsOnPytestFailure(t *testing.T) {
	runtime := t.TempDir()
	venvBin := filepath.Join(runtime, ".venv", "bin")
	if err := os.MkdirAll(venvBin, 0o755); err != nil {
		t.Fatal(err)
	}
	pytest := filepath.Join(venvBin, "pytest")
	if err := os.WriteFile(pytest, []byte("#!/bin/sh\nexit 9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runInfraUnit(runtime, t.TempDir(), "", strings.NewReader(""), &stdout, &stderr); code != 9 {
		t.Fatalf("unit exit code = %d, want 9", code)
	}
}

func TestInfraUnitPatternUsesPluginFilter(t *testing.T) {
	command := infraUnitCommand("/runtime", "auth login")
	if !sameStrings(command[6:10], []string{
		"-p", "_pytest_pattern", "--leedevkit-pattern", "auth login",
	}) {
		t.Fatalf("pattern args = %#v", command[6:10])
	}
	if contains(command, "--cov-fail-under=80") {
		t.Fatal("pattern run must not claim full coverage")
	}
}

func TestInfraUnitPatternRejectsUnsafeRegex(t *testing.T) {
	if _, err := validateInfraPattern("["); err == nil {
		t.Fatal("invalid regex accepted")
	}
	if _, err := validateInfraPattern("auth\x00"); err == nil {
		t.Fatal("control character accepted")
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

func TestRuntimeRootFindsScriptsAncestor(t *testing.T) {
	runtime := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runtime, "bin", "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runtime, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "scripts", "_orchestrator.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "VERSION"), []byte("0.11.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runtimeRootFor(filepath.Join(runtime, "bin", "go", "leedevkit-go-linux-amd64"), "", nil)
	if got != runtime {
		t.Fatalf("runtime root = %q, want %q", got, runtime)
	}
}

func TestRunPythonCommandMissingRuntime(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runPythonCommand([]string{"test"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("missing runtime exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "Python compatibility runtime missing") {
		t.Fatalf("missing runtime stderr = %q", stderr.String())
	}
}

func TestRuntimeRootPrefersExplicitRuntime(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	projectRuntime := filepath.Join(project, ".leedevkit")
	explicitRuntime := filepath.Join(root, "explicit")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, runtime := range []string{projectRuntime, explicitRuntime} {
		if err := os.MkdirAll(filepath.Join(runtime, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runtime, "scripts", "_orchestrator.py"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runtime, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "leedevkit.toml"), []byte("[devkit]\nversion = \"v1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runtimeRootFor(filepath.Join(root, "bin", "leedevkit"), project, map[string]string{"DEVKIT_HOME": explicitRuntime})
	if got != explicitRuntime {
		t.Fatalf("runtime root = %q, want explicit runtime %q", got, explicitRuntime)
	}
}

func TestRuntimeRootPrefersMatchingProjectRuntime(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project with spaces")
	projectRuntime := filepath.Join(project, ".leedevkit")
	globalRuntime := filepath.Join(root, "global", "current")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, runtime := range []string{projectRuntime, globalRuntime} {
		if err := os.MkdirAll(filepath.Join(runtime, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runtime, "scripts", "_orchestrator.py"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "leedevkit.toml"), []byte("[devkit]\nversion = \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRuntime, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalRuntime, "VERSION"), []byte("9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runtimeRootFor(filepath.Join(globalRuntime, "bin", "leedevkit"), project, nil)
	if got != projectRuntime {
		t.Fatalf("runtime root = %q, want project runtime %q", got, projectRuntime)
	}
}

func TestRuntimeRootUsesInstalledRuntimeForPinnedProject(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	runtime := filepath.Join(root, "install", "current")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runtime, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "scripts", "_orchestrator.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "leedevkit.toml"), []byte("[devkit]\nversion = \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runtimeRootFor(filepath.Join(runtime, "bin", "leedevkit"), project, nil)
	if got != runtime {
		t.Fatalf("runtime root = %q, want installed runtime %q", got, runtime)
	}
}

func TestRuntimeRootRejectsMismatchedExplicitRuntime(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	runtime := filepath.Join(root, "runtime")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runtime, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "scripts", "_orchestrator.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "VERSION"), []byte("9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "leedevkit.toml"), []byte("[devkit]\nversion = \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runtimeRootFor(filepath.Join(root, "bin", "leedevkit"), project, map[string]string{"DEVKIT_HOME": runtime})
	if got != "" {
		t.Fatalf("runtime root = %q, want no compatible runtime", got)
	}
}
