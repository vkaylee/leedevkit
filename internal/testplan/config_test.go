package testplan

import (
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
)

func TestRequestFromConfigUsesServicesTargets(t *testing.T) {
	request := RequestFromConfig("all", config.Project{Services: map[string]config.Service{"go": {Language: "go"}}, Targets: map[string][]string{"all": {"go"}, "backend": {"go"}}})
	result, err := Plan(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 1 || result.Targets[0] != "backend" || result.Modes["go"] != "go" {
		t.Fatalf("result = %#v", result)
	}
}
