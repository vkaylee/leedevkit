package provider

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveUsesPrecedence(t *testing.T) {
	repository := writeConfig(t, `[container]
provider = "repo"

[container.providers.repo]
cli = ["repo-engine"]
compose = ["repo-engine", "compose"]

[container.providers.host]
cli = ["host-engine"]
compose = ["host-engine", "compose"]
`)
	host := writeConfig(t, `[container]
provider = "host"
`)
	provider, err := Resolve(Options{
		Repository: repository,
		Host:       host,
		Environment: map[string]string{
			"CONTAINER_PROVIDER": "host",
		},
		CLI: "repo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Name != "repo" || provider.Source != "CLI override" {
		t.Fatalf("provider = %#v", provider)
	}
	if !reflect.DeepEqual(provider.Compose, []string{"repo-engine", "compose"}) {
		t.Fatalf("compose = %#v", provider.Compose)
	}
}

func TestResolveDefaultsDocker(t *testing.T) {
	provider, err := Resolve(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Name != "docker" {
		t.Fatalf("provider name = %q", provider.Name)
	}
	if !reflect.DeepEqual(provider.Compose, []string{"docker", "compose"}) {
		t.Fatalf("compose = %#v", provider.Compose)
	}
}

func TestResolveRejectsUnknownCapability(t *testing.T) {
	repository := writeConfig(t, `[container]
provider = "custom"

[container.providers.custom]
cli = ["custom"]
compose = ["custom", "compose"]
capabilities = ["teleport"]
`)
	_, err := Resolve(Options{Repository: repository})
	if err == nil {
		t.Fatal("expected unknown capability error")
	}
}

func TestDiagnosticsRedactsSecrets(t *testing.T) {
	repository := writeConfig(t, `[container]
provider = "docker"

[container.providers.docker]
cli = ["docker"]
compose = ["docker", "compose"]
password = "secret"
endpoint = "https://user:pass@example.test/socket"
`)
	provider, err := Resolve(Options{Repository: repository})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := Diagnostics(provider)
	settings := diagnostics["settings"].(map[string]any)
	if settings["password"] != "<redacted>" {
		t.Fatalf("password = %q", settings["password"])
	}
	if settings["endpoint"] != "https://example.test/socket" {
		t.Fatalf("endpoint = %q", settings["endpoint"])
	}
}
