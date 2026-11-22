package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTestRejectsMissingTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"test"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("test exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "test target is required") {
		t.Fatalf("test stderr = %q", stderr.String())
	}
}

func TestRunTestRejectsInvalidProjectConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[project\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	chdirForTest(t, root)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"test", "all"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("test exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid project config:") {
		t.Fatalf("test stderr = %q", stderr.String())
	}
}

func TestRunTestRejectsBaseRefWithoutValue(t *testing.T) {
	chdirForTest(t, t.TempDir())

	var stdout, stderr bytes.Buffer
	if code := run([]string{"test", "all", "--base-ref"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("test exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--base-ref requires a value") {
		t.Fatalf("test stderr = %q", stderr.String())
	}
}

func TestPlanLifecycleRejectsUnsupportedOperation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	input := `{"schema_version":1,"operation":"restart"}`
	if code := run([]string{"--plan-lifecycle"}, strings.NewReader(input), &stdout, &stderr); code != 2 {
		t.Fatalf("lifecycle exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unsupported lifecycle operation "restart"`) {
		t.Fatalf("lifecycle stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("lifecycle stdout = %q, want empty", stdout.String())
	}
}

func chdirForTest(t *testing.T, directory string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}
