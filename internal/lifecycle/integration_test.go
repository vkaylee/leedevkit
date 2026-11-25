package lifecycle

import (
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/provider"
)

func TestPlanBuildsUpAndDownForComposeExecutor(t *testing.T) {
	plan := Plan{
		Provider: provider.Provider{Name: "docker", Compose: []string{"docker", "compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}},
		Project:  "leedevkit-test", Files: []string{"compose.yml"}, Profiles: []string{"--profile", "go"}, Services: []string{"go"}, Build: false, OwnsProject: true,
	}
	up, err := plan.Up()
	if err != nil {
		t.Fatal(err)
	}
	down, err := plan.Down()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(up, []string{"docker", "compose", "-p", "leedevkit-test", "-f", "compose.yml", "--profile", "go", "up", "-d", "go"}) {
		t.Fatalf("up = %#v", up)
	}
	if !reflect.DeepEqual(down, []string{"docker", "compose", "-p", "leedevkit-test", "-f", "compose.yml", "down", "--remove-orphans", "--timeout", "1"}) {
		t.Fatalf("down = %#v", down)
	}
}
