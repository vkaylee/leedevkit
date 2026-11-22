package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsSymlinkedAgentAndMissingAssets(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".agent")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	var output bytes.Buffer
	if code := runDoctorWithAssets(root, "", &bytes.Buffer{}, &output); code != 0 {
		t.Fatalf("doctor exit = %d, output = %s", code, output.String())
	}
	for _, want := range []string{"leedevkit.toml", ".agent is a symlink", "global DevKit assets unavailable", "runtime docker: unavailable"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing %q in %s", want, output.String())
		}
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("doctor modified symlink target: %v, %v", entries, err)
	}
}

func TestRepairDoctorRulesRejectsMissingAssetsWithoutCreatingRules(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	if err := repairDoctorRulesFromAssets(root, assets); err == nil || !strings.Contains(err.Error(), "global rules unavailable") {
		t.Fatalf("repair error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".agent")); !os.IsNotExist(err) {
		t.Fatalf("repair changed project: %v", err)
	}
}
