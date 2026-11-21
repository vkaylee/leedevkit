package provider

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveConformancePrecedenceAndDefaults(t *testing.T) {
	dir := t.TempDir()
	repository := filepath.Join(dir, "repo.toml")
	host := filepath.Join(dir, "host.toml")
	if err := os.WriteFile(repository, []byte(`[container]
provider = "repo"
[container.providers.repo]
cli = ["repo"]
compose = ["repo", "compose"]
capabilities = ["compose", "config", "up", "down", "ps"]
[container.providers.host]
cli = ["host"]
compose = ["host", "compose"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(host, []byte(`[container]
provider = "host"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(Options{Repository: repository, Host: host, Environment: map[string]string{"CONTAINER_PROVIDER": "repo"}, CLI: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "repo" || !reflect.DeepEqual(got.Compose, []string{"repo", "compose"}) {
		t.Fatalf("provider = %#v", got)
	}
}

func TestResolveConformanceUnknownProviderFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo.toml")
	if err := os.WriteFile(path, []byte(`[container]
provider = "missing"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(Options{Repository: path}); err == nil {
		t.Fatal("expected missing provider error")
	}
}
