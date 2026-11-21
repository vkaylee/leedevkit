package provider

import "testing"

func TestHostConfigPathUsesXDG(t *testing.T) {
	got := HostConfigPath(map[string]string{"XDG_CONFIG_HOME": "/tmp/config"}, "/home/user")
	want := "/tmp/config/leedevkit/config.toml"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestHostConfigPathUsesHomeFallback(t *testing.T) {
	got := HostConfigPath(map[string]string{}, "/home/user")
	want := "/home/user/.config/leedevkit/config.toml"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
