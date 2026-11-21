package main

import (
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
)

func TestShouldUseGoTestUsesGoForInfra(t *testing.T) {
	if !shouldUseGoTest(config.Project{}, "infra") {
		t.Fatal("infra dispatch should use Go")
	}
}

func TestShouldUseGoTestRequiresConfiguredGoTarget(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{"backend": {Language: "go"}}, Targets: map[string][]string{"all": {"go"}}}
	if !shouldUseGoTest(project, "go") {
		t.Fatal("configured Go target should use Go")
	}
	if !shouldUseGoTest(project, "all") {
		t.Fatal("Go-only all target should use Go")
	}
}

func TestShouldUseGoTestRejectsUnsupportedTargets(t *testing.T) {
	goProject := config.Project{Services: map[string]config.Service{"backend": {Language: "go"}}, Targets: map[string][]string{"all": {"go", "web"}}}
	if shouldUseGoTest(goProject, "all") {
		t.Fatal("mixed target must not claim Go parity")
	}
}
