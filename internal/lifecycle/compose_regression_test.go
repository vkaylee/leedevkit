package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestComposeExecutorStreamsOutputAndReturnsExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	failure := errors.New("command failed")
	executor := ComposeExecutor{Stdout: &stdout, Stderr: &stderr, Run: func(_ context.Context, request process.Request) process.Result {
		fmt.Fprint(request.Stdout, "output")
		fmt.Fprint(request.Stderr, "error")
		return process.Result{ExitCode: 7, Err: failure}
	}}
	err := executor.Execute(context.Background(), []string{"compose", "up"})
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.ExitCode != 7 || !errors.Is(err, failure) || stdout.String() != "output" || stderr.String() != "error" || !strings.Contains(err.Error(), "error") {
		t.Fatalf("error=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if err := executor.Execute(context.Background(), nil); err == nil {
		t.Fatal("empty compose command accepted")
	}
}
