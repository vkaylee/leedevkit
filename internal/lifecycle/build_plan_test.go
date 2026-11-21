package lifecycle

import (
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/provider"
)

func TestPlanBuildArgsUsesSameComposeBoundary(t *testing.T) {
	plan := Plan{
		Provider: provider.Provider{Name: "docker", Compose: []string{"docker", "compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps", "build"}},
		Project:  "test", Files: []string{"compose.yml"}, Profiles: []string{"--profile", "go"}, Services: []string{"go"}, OwnsProject: true,
	}
	args, err := plan.BuildArgs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "-p", "test", "-f", "compose.yml", "--profile", "go", "build", "go"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestPlanBuildArgsRequiresBuildCapability(t *testing.T) {
	plan := Plan{Provider: provider.Provider{Name: "docker", Compose: []string{"docker", "compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}, Project: "test", OwnsProject: true}
	if _, err := plan.BuildArgs(); err == nil {
		t.Fatal("expected missing build capability")
	}
}
