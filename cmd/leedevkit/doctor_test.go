package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDoctorReadOnlyReportsProjectState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[project]\nname = \"test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent", "rules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent", "rules", "coding.md"), []byte("# coding"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".leedevkit", "VERSION"), []byte("0.11.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if code := runDoctor(root, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("doctor exit code = %d, output = %q", code, stderr.String())
	}
	for _, want := range []string{"Project config:", ".agent: real directory, 1 rulebooks", "DevKit:", "runtime docker:"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("doctor output missing %q: %s", want, stderr.String())
		}
	}
}

func TestRunDoctorDoesNotModifyProject(t *testing.T) {
	root := t.TempDir()
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := runDoctor(root, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("doctor exit code = %d", code)
	}
	after, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("doctor mutated project: before=%d after=%d", len(before), len(after))
	}
}
