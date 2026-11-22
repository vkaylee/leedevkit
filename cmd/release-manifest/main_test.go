package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestCommand(t *testing.T) {
	if os.Getenv("TEST_MANIFEST_HELPER") == "1" {
		os.Args = append([]string{"release-manifest"}, os.Args[3:]...)
		main()
		return
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestManifestCommand", "--"}, args...)...)
		cmd.Env = append(os.Environ(), "TEST_MANIFEST_HELPER=1")
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	if output, err := run("-root", root); err != nil {
		t.Fatalf("write: %v: %s", err, output)
	}
	if output, err := run("-root", root, "-verify"); err != nil {
		t.Fatalf("verify: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := run("-root", root, "-verify"); err == nil || !strings.Contains(output, "manifest verification failed") {
		t.Fatalf("modified release accepted: %v: %s", err, output)
	}
	if output, err := run("-root", filepath.Join(root, "missing")); err == nil || !strings.Contains(output, "VERSION") {
		t.Fatalf("missing VERSION accepted: %v: %s", err, output)
	}
}
