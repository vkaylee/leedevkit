package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndVerifyManifestExcludesRuntimeState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("0.11.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".venv"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".venv", "generated"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dev-state.json"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(root); err != nil {
		t.Fatal(err)
	}
	result, err := VerifyManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Clean() || result.FileCount != 2 {
		t.Fatalf("verification = %#v", result)
	}
}

func TestVerifyManifestDetectsDriftAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "VERSION")); err != nil {
		t.Fatal(err)
	}
	result, err := VerifyManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Clean() || len(result.Modified) != 1 || len(result.Missing) != 1 {
		t.Fatalf("verification = %#v", result)
	}
}
