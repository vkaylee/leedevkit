package lifecycle

import (
	"context"
	"errors"
	"testing"

	"github.com/vkaylee/leedevkit/internal/provider"
)

func testProviderWithBuild() provider.Provider {
	return provider.Provider{Name: "docker", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps", "build"}}
}

func TestBuildFlowRunsPlanBuildAndPublishesMetadata(t *testing.T) {
	calls := []string{}
	flow := BuildFlow{Runner: BuildRunner{Lock: NewBuildLock(), Execute: func(_ context.Context, args []string) error { calls = append(calls, args[0]); return nil }, Verify: func(context.Context, string, string) error { calls = append(calls, "verify"); return nil }, Publish: func(BuildMetadata) error { calls = append(calls, "publish"); return nil }}}
	plan := Plan{Provider: testProviderWithBuild(), Project: "test", Profiles: []string{"--profile", "go"}, Services: []string{"go"}, OwnsProject: true}
	if err := flow.Run(context.Background(), plan, "fp", "docker", "image"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[1] != "verify" || calls[2] != "publish" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestBuildFlowDoesNotPublishOnBuildFailure(t *testing.T) {
	published := false
	flow := BuildFlow{Runner: BuildRunner{Execute: func(context.Context, []string) error { return errors.New("failed") }, Publish: func(BuildMetadata) error { published = true; return nil }}}
	plan := Plan{Provider: testProviderWithBuild(), Project: "test", Services: []string{"go"}, OwnsProject: true}
	if err := flow.Run(context.Background(), plan, "fp", "docker", "image"); err == nil {
		t.Fatal("expected error")
	}
	if published {
		t.Fatal("published after build failure")
	}
}
