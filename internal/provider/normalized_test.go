package provider

import "testing"

func TestNormalizeAndRequire(t *testing.T) {
	p, err := Normalize(ProtocolProvider{Name: "docker", CLI: []string{"docker"}, Compose: []string{"docker", "compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Require(p, "compose", "up", "down"); err != nil {
		t.Fatal(err)
	}
}

func TestRequireRejectsMissingCapability(t *testing.T) {
	p, err := Normalize(ProtocolProvider{Name: "custom", CLI: []string{"custom"}, Compose: []string{"custom", "compose"}, Capabilities: []string{"compose"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Require(p, "wait"); err == nil {
		t.Fatal("expected missing capability")
	}
}
