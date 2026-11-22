package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestDevRunnerReportsFailureAndReturnsExitCode(t *testing.T) {
	var stderr bytes.Buffer
	failure := errors.New("fixture failure")
	code := runDev("up", testProviderForMain(), t.TempDir(), &bytes.Buffer{}, &stderr, func(_ context.Context, request process.Request) process.Result {
		if len(request.Args) == 0 || request.Args[len(request.Args)-1] != "-d" {
			t.Fatalf("args = %q", request.Args)
		}
		return process.Result{ExitCode: 9, Err: failure}
	})
	if code != 9 || !strings.Contains(stderr.String(), failure.Error()) {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}
