package lifecycle

import (
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/provider"
)

func TestPlanUsesFreshMetadataWithoutBuild(t *testing.T) {
	plan := Plan{Provider: provider.Provider{Name: "docker", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}, Project: "test", Services: []string{"go"}, BuildPolicy: BuildPolicy{}, BuildMetadata: BuildMetadata{Expected: "same", Actual: "same"}, OwnsProject: true}
	args, err := plan.Up()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args[len(args)-2:], []string{"--no-build", "go"}) {
		t.Fatalf("args = %#v", args)
	}
}

func TestPlanBuildsWhenMetadataStale(t *testing.T) {
	plan := Plan{Provider: provider.Provider{Name: "docker", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}, Project: "test", Services: []string{"go"}, BuildPolicy: BuildPolicy{}, BuildMetadata: BuildMetadata{Expected: "new", Actual: "old"}, OwnsProject: true}
	args, err := plan.Up()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args[len(args)-2:], []string{"--build", "go"}) {
		t.Fatalf("args = %#v", args)
	}
}
