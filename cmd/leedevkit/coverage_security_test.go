package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoveragePatternIsPassedAsLiteralArgument(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "injected")
	log := filepath.Join(root, "argv")
	fakeGo := filepath.Join(root, "go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$ARG_LOG\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mkdir"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	pattern := "TestExample;touch " + marker
	args, err := buildGoCoverageCommand([]string{"compose"}, "go", pattern, false)
	if err != nil {
		t.Fatal(err)
	}
	index := -1
	for i, arg := range args {
		if arg == "-c" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("shell command missing: %q", args)
	}
	cmd := exec.Command("sh", append([]string{"-c", args[index+1]}, args[index+2:]...)...)
	cmd.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "ARG_LOG="+log)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("command error=%v output=%q", err, output.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("pattern executed shell command: %v", err)
	}
	data, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(data), "-run="+pattern+"\n") {
		t.Fatalf("go argv=%q error=%v", data, err)
	}
}
