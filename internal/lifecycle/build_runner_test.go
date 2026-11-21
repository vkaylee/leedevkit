package lifecycle

import (
	"context"
	"errors"
	"testing"
)

func TestBuildRunnerPublishesMetadataAfterSuccess(t *testing.T) {
	published := false
	runner := BuildRunner{Lock: NewBuildLock(), Execute: func(context.Context, []string) error { return nil }, Verify: func(context.Context, string, string) error { return nil }, Publish: func(BuildMetadata) error { published = true; return nil }}
	if err := runner.Run(context.Background(), "fingerprint", []string{"compose", "build"}, BuildMetadata{Expected: "fingerprint"}); err != nil {
		t.Fatal(err)
	}
	if !published {
		t.Fatal("metadata was not published")
	}
}

func TestBuildRunnerDoesNotPublishAfterFailure(t *testing.T) {
	published := false
	runner := BuildRunner{Lock: NewBuildLock(), Execute: func(context.Context, []string) error { return errors.New("build failed") }, Verify: func(context.Context, string, string) error { return nil }, Publish: func(BuildMetadata) error { published = true; return nil }}
	if err := runner.Run(context.Background(), "fingerprint", []string{"compose", "build"}, BuildMetadata{Expected: "fingerprint"}); err == nil {
		t.Fatal("expected build error")
	}
	if published {
		t.Fatal("stale metadata published")
	}
}
