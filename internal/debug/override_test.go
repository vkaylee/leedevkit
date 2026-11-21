package debug

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOverrideHardensDebugContainer(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	logPath := filepath.Join(root, "api.log")
	for path, data := range map[string]string{manifest: "debug = {}\n", logPath: "safe\n"} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	environment := Environment{Name: "fixture", ComposeProject: "fixture", ProjectRoot: root, SourcePath: manifest, Services: map[string]Service{"api": {ID: "api", LogFiles: []string{"api.log"}}}}
	directory := t.TempDir()
	path, err := WriteOverride(environment, []string{"fixture_backend"}, directory, "leedevkit-debug:fixture")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	service := document["services"].(map[string]any)["leedevkit-debug"].(map[string]any)
	build := service["build"].(map[string]any)
	if build["context"] != root || build["dockerfile"] != "container/debug/Dockerfile" {
		t.Fatalf("build = %#v", build)
	}
	if service["read_only"] != true || service["user"] != "65532:65532" {
		t.Fatalf("service hardening = %#v", service)
	}
	if service["cap_drop"].([]any)[0] != "ALL" || service["security_opt"].([]any)[0] != "no-new-privileges:true" {
		t.Fatalf("security hardening = %#v", service)
	}
	mounts := service["volumes"].([]any)
	if len(mounts) != 2 {
		t.Fatalf("mounts = %#v", mounts)
	}
	for _, mount := range mounts {
		if filepath.Ext(mount.(string)) == ".rw" || !containsString(mount.(string), ":ro") {
			t.Fatalf("unsafe mount = %q", mount)
		}
	}
}

func TestWriteOverrideRejectsUnsafeLogSource(t *testing.T) {
	root := t.TempDir()
	environment := Environment{Name: "fixture", ProjectRoot: root, SourcePath: filepath.Join(root, "manifest.toml"), Services: map[string]Service{"api": {ID: "api", LogFiles: []string{"../secret"}}}}
	if _, err := WriteOverride(environment, []string{"backend"}, t.TempDir(), "image"); err == nil {
		t.Fatal("unsafe log source accepted")
	}
}

func containsString(value, wanted string) bool {
	for _, part := range []string{wanted} {
		if value == part || len(value) >= len(part) && value[len(value)-len(part):] == part {
			return true
		}
	}
	return false
}
