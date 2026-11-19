package main

import (
	"bytes"
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
	got := runtimeRoot(filepath.Join(runtime, "bin", "go", "leedevkit-go-linux-amd64"))
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
