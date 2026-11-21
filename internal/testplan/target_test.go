package testplan

import (
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
)

func TestRequestFromConfigIsDeterministic(t *testing.T) {
	request := RequestFromConfig("all", config.Project{Services: map[string]config.Service{"z": {Language: "go"}, "a": {Language: "rust"}}, Targets: map[string][]string{"all": {"z", "a"}, "z-target": {"z"}, "a-target": {"a"}}})
	if request.Services[0].Name != "a" || request.Services[1].Name != "z" {
		t.Fatalf("services = %#v", request.Services)
	}
	if request.Aliases[0].Name != "a-target" || request.Aliases[1].Name != "z-target" {
		t.Fatalf("aliases = %#v", request.Aliases)
	}
}
