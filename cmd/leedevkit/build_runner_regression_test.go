package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRunnerRunsInProjectAndPublishesMetadata(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "builder")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s:%s' \"$PWD\" \"$LEEDEVKIT_PROJECT_ROOT\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	runner := newBuildRunner(root, "fixture", &output)
	if err := runner.Execute(context.Background(), []string{script}); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != root+":"+root {
		t.Fatalf("build output = %q", got)
	}
	if err := runner.Execute(context.Background(), []string{filepath.Join(root, "missing")}); err == nil {
		t.Fatal("missing builder accepted")
	}
	if !strings.Contains(output.String(), root) {
		t.Fatal("build output missing project path")
	}
}
