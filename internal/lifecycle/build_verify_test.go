package lifecycle

import (
	"context"
	"errors"
	"testing"
)

func TestBuildRunnerDoesNotPublishOnImageMismatch(t *testing.T) {
	published := false
	runner := BuildRunner{Lock: NewBuildLock(), Execute: func(context.Context, []string) error { return nil }, Verify: func(context.Context, string, string) error { return errors.New("fingerprint mismatch") }, Publish: func(BuildMetadata) error { published = true; return nil }}
	if err := runner.Run(context.Background(), "fp", []string{"compose", "build"}, BuildMetadata{}); err == nil {
		t.Fatal("expected verify error")
	}
	if published {
		t.Fatal("metadata published after mismatch")
	}
}
