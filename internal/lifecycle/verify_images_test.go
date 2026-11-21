package lifecycle

import (
	"context"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestVerifyImagesChecksEveryImage(t *testing.T) {
	seen := []string{}
	err := VerifyImages(context.Background(), func(_ context.Context, _ process.Request) process.Result {
		return process.Result{ExitCode: 0, Stdout: []byte(`{"com.leedevkit.build-fingerprint":"fp"}`)}
	}, "docker", []string{"one", "two"}, "fp", func(image string) { seen = append(seen, image) })
	if err != nil || len(seen) != 2 {
		t.Fatalf("err=%v seen=%v", err, seen)
	}
}

func TestVerifyImagesStopsOnMismatch(t *testing.T) {
	err := VerifyImages(context.Background(), func(_ context.Context, request process.Request) process.Result {
		if request.Args[len(request.Args)-1] == "bad" {
			return process.Result{ExitCode: 0, Stdout: []byte(`{"com.leedevkit.build-fingerprint":"old"}`)}
		}
		return process.Result{ExitCode: 0, Stdout: []byte(`{"com.leedevkit.build-fingerprint":"fp"}`)}
	}, "docker", []string{"good", "bad"}, "fp", nil)
	if err == nil {
		t.Fatal("expected mismatch")
	}
}
