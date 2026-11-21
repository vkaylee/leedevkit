package provider

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveHostOverridesOnlySelectedNestedProviderFields(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo.toml")
	host := filepath.Join(dir, "host.toml")
	if err := os.WriteFile(repo, []byte(`[container]
provider = "docker"
[container.providers.docker]
cli = ["docker"]
compose = ["docker", "compose"]
capabilities = ["compose", "config", "up", "down", "ps"]
endpoint = "https://repo.example/socket"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(host, []byte(`[container.providers.docker]
endpoint = "https://user:pass@host.example/socket"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(Options{Repository: repo, Host: host})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.CLI, []string{"docker"}) || !reflect.DeepEqual(got.Compose, []string{"docker", "compose"}) {
		t.Fatalf("provider = %#v", got)
	}
	if got.Settings["endpoint"] != "https://user:pass@host.example/socket" {
		t.Fatalf("settings = %#v", got.Settings)
	}
}
