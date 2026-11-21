package debug

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, content string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func TestLoadEnvironmentValidatesSafeReadOnlyManifest(t *testing.T) {
	root, manifest := writeManifest(t, `[debug]
manifest_version = 1

[debug.environments.fixture]
compose_project = "fixture-project"
compose_files = ["compose.yml"]
profiles = ["debug"]
approved_networks = ["fixture_backend"]

[debug.environments.fixture.services.api]
aliases = ["api"]
internal_host = "api"
internal_port = 8080
health_paths = ["/health"]
http_get_paths = ["/status"]
log_sources = []

[debug.environments.fixture.database]
enabled = false
`)
	environment, err := Load(manifest, "fixture", Options{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if environment.Name != "fixture" || environment.ComposeProject != "fixture-project" || environment.Services["api"].Port != 8080 {
		t.Fatalf("environment = %#v", environment)
	}
	if environment.Metadata()["mode"] != "read-only" {
		t.Fatalf("metadata = %#v", environment.Metadata())
	}
}

func TestLoadRejectsProductionAndUnsafeHost(t *testing.T) {
	root, manifest := writeManifest(t, `[debug]
manifest_version = 1

[debug.environments.production]
compose_project = "safe-project"
compose_files = ["compose.yml"]
approved_networks = ["backend"]

[debug.environments.production.services.api]
internal_host = "localhost"
internal_port = 8080
`)
	if _, err := Load(manifest, "production", Options{ProjectRoot: root}); err == nil {
		t.Fatal("production environment accepted")
	}
}

func TestLoadRejectsPathEscapeAndMetadataHost(t *testing.T) {
	root, manifest := writeManifest(t, `[debug]
manifest_version = 1

[debug.environments.fixture]
compose_project = "safe-project"
compose_files = ["../compose.yml"]
approved_networks = ["backend"]

[debug.environments.fixture.services.api]
internal_host = "169.254.169.254"
internal_port = 8080
`)
	if _, err := Load(manifest, "fixture", Options{ProjectRoot: root}); err == nil {
		t.Fatal("unsafe manifest accepted")
	}
}

func TestLoadRejectsInvalidRedactionPattern(t *testing.T) {
	root, manifest := writeManifest(t, `[debug]
manifest_version = 1

[debug.environments.fixture]
compose_project = "safe-project"
compose_files = ["compose.yml"]
approved_networks = ["backend"]

[debug.environments.fixture.services.api]
internal_host = "api"
internal_port = 8080

[debug.environments.fixture.redaction]
patterns = ["["]
`)
	if _, err := Load(manifest, "fixture", Options{ProjectRoot: root}); err == nil {
		t.Fatal("invalid redaction pattern accepted")
	}
}
