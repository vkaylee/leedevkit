package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestComposeExecutorPropagatesSuccess(t *testing.T) {
	executor := ComposeExecutor{Run: func(_ context.Context, request process.Request) process.Result {
		if len(request.Args) != 2 || request.Args[1] != "up" {
			t.Fatalf("args = %#v", request.Args)
		}
		return process.Result{}
	}}
	if err := executor.Execute(context.Background(), []string{"compose", "up"}); err != nil {
		t.Fatal(err)
	}
}

func TestComposeExecutorPropagatesFailure(t *testing.T) {
	want := errors.New("compose failed")
	executor := ComposeExecutor{Run: func(context.Context, process.Request) process.Result { return process.Result{ExitCode: 7, Err: want} }}
	if err := executor.Execute(context.Background(), []string{"compose", "up"}); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func TestComposeExecutorIncludesCapturedStderrAndRequestDirectory(t *testing.T) {
	executor := ComposeExecutor{
		Dir: "/project",
		Env: []string{"PROJECT_ROOT=/project"},
		Run: func(_ context.Context, request process.Request) process.Result {
			if request.Dir != "/project" || len(request.Env) != 1 {
				t.Fatalf("dir = %q", request.Dir)
			}
			return process.Result{ExitCode: 1, Stderr: []byte("compose: no such service api\n")}
		},
	}
	if err := executor.Execute(context.Background(), []string{"docker", "compose", "up"}); err == nil || !strings.Contains(err.Error(), "no such service api") {
		t.Fatalf("error = %v", err)
	}
}
