package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectEnvironmentReplacesOnlyProjectRoot(t *testing.T) {
	original := []string{"TOKEN=a=b", "LEEDEVKIT_PROJECT_ROOT=/old", "OTHER=value", "LEEDEVKIT_PROJECT_ROOT=/other"}
	got := withProjectRoot(original, "/new")
	want := []string{"TOKEN=a=b", "OTHER=value", "LEEDEVKIT_PROJECT_ROOT=/new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %q, want %q", got, want)
	}
	if original[1] != "LEEDEVKIT_PROJECT_ROOT=/old" {
		t.Fatal("input environment mutated")
	}
	parsed := environMap(append(got, "NOT_AN_ASSIGNMENT"))
	if parsed["TOKEN"] != "a=b" || parsed["LEEDEVKIT_PROJECT_ROOT"] != "/new" {
		t.Fatalf("parsed environment = %#v", parsed)
	}
	if _, ok := parsed["NOT_AN_ASSIGNMENT"]; ok {
		t.Fatal("invalid environment entry accepted")
	}
}

func TestRunCommandPreservesOutputAndExitCode(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runCommand("unused", root, []string{"sh", "-c", `pwd; printf 'diagnostic' >&2; exit 7`}, strings.NewReader(""), &stdout, &stderr)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 || strings.TrimSpace(stdout.String()) != resolved || stderr.String() != "diagnostic" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runCommand("unused", root, []string{filepath.Join(root, "missing-command")}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("missing command exit=%d, want 2", code)
	}
}

func TestProjectRootDiscoveryFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if got := projectRootOrCWD(); got != root {
		t.Fatalf("project root = %q, want %q", got, root)
	}
}
