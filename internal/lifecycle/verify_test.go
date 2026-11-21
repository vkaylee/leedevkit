package lifecycle

import (
	"context"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestVerifyImageFingerprint(t *testing.T) {
	verified := false
	err := VerifyImageFingerprint(context.Background(), func(context.Context, process.Request) process.Result {
		verified = true
		return process.Result{ExitCode: 0, Stdout: []byte(`{"com.leedevkit.build-fingerprint":"abc"}`)}
	}, "docker", "image", "abc")
	if err != nil || !verified {
		t.Fatalf("err=%v verified=%v", err, verified)
	}
}

func TestVerifyImageFingerprintRejectsMismatch(t *testing.T) {
	err := VerifyImageFingerprint(context.Background(), func(context.Context, process.Request) process.Result {
		return process.Result{ExitCode: 0, Stdout: []byte(`{"com.leedevkit.build-fingerprint":"old"}`)}
	}, "docker", "image", "abc")
	if err == nil {
		t.Fatal("expected mismatch")
	}
}
