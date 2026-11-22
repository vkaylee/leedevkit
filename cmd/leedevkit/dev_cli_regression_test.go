package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevExecUsesConfiguredEngineAndPropagatesFailure(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "compose-fixture")
	logPath := filepath.Join(root, "argv.log")
	if err := os.WriteFile(engine, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGV_LOG\"\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("ARGV_LOG", logPath)
	t.Setenv("HOME", root)
	config := "[container]\nprovider = 'fixture'\n[container.providers.fixture]\ncli = ['fixture-engine']\ncompose = ['" + engine + "']\n"
	if err := os.WriteFile("leedevkit.toml", []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"dev", "exec", "api", "go", "test"}, strings.NewReader(""), &stdout, &stderr); code != 7 {
		t.Fatalf("dev exec exit = %d, stderr = %q", code, stderr.String())
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-p\nleedevkit-dev\n", "exec\napi\ngo\ntest\n"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("dev exec args missing %q: %q", want, got)
		}
	}
}
