package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
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
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version exit code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "0.10.0-test" {
		t.Fatalf("version = %q", got)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"wat"}, &stdout, &stderr); code != 1 {
		t.Fatalf("unknown command exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "Unknown command: wat") {
		t.Fatalf("unknown command stderr = %q", stderr.String())
	}
}

func TestRunUnimplementedCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"test"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unimplemented command exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "Go core command not implemented: test") {
		t.Fatalf("unimplemented stderr = %q", stderr.String())
	}
}
