package testplan

import (
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
)

func TestExpandTargetsPreservesConfiguredAllOrder(t *testing.T) {
	request := Request{
		Target:      "all",
		AllServices: []string{"api", "web", "standalone"},
		Aliases:     []Alias{{Name: "web", Services: []string{"web"}}, {Name: "api", Services: []string{"api"}}},
	}
	got, err := ExpandTargets(request)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"web", "api", "standalone"}
	if len(got) != len(want) {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targets = %#v, want %#v", got, want)
		}
	}
}

func TestRequestFromConfigIsDeterministic(t *testing.T) {
	request := RequestFromConfig("all", config.Project{Services: map[string]config.Service{"z": {Language: "go"}, "a": {Language: "rust"}}, Targets: map[string][]string{"all": {"z", "a"}, "z-target": {"z"}, "a-target": {"a"}}})
	if request.Services[0].Name != "a" || request.Services[1].Name != "z" {
		t.Fatalf("services = %#v", request.Services)
	}
	if request.Aliases[0].Name != "a-target" || request.Aliases[1].Name != "z-target" {
		t.Fatalf("aliases = %#v", request.Aliases)
	}
}
