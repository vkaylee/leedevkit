package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSkillsListIsReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit", ".agent", "skills", "testing"), 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := runSkillsList(root, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("skills list exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "testing") {
		t.Fatalf("skills output = %q", stderr.String())
	}
	after, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatal("skills list mutated project")
	}
}
