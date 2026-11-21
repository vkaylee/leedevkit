package run

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestExecuteRunsCommandAndOwnedCleanupOnce(t *testing.T) {
	var commandArgs, cleanupArgs []string
	result := Execute(context.Background(), ExecuteRequest{
		Plan: Request{
			Compose: []string{"compose"}, Project: "leedevkit-test-execute",
			Tool: "go", Args: []string{"version"}, Service: "go",
		},
		Owned: true, CleanupBase: []string{"compose", "-p", "leedevkit-test-execute"},
		RunProcess: func(_ context.Context, request process.Request) process.Result {
			commandArgs = append([]string(nil), request.Args...)
			return process.Result{ExitCode: 0}
		},
		RunCompose: func(args []string) error {
			cleanupArgs = append([]string(nil), args...)
			return nil
		},
	})
	if result.Err != nil || result.Process.ExitCode != 0 {
		t.Fatalf("result = %#v", result)
	}
	if len(commandArgs) == 0 || len(cleanupArgs) == 0 {
		t.Fatalf("command args = %#v, cleanup args = %#v", commandArgs, cleanupArgs)
	}
	wantCleanup := []string{"compose", "-p", "leedevkit-test-execute", "down", "--remove-orphans", "--timeout", "1"}
	if !reflect.DeepEqual(cleanupArgs, wantCleanup) {
		t.Fatalf("cleanup args = %#v, want %#v", cleanupArgs, wantCleanup)
	}
}

func TestExecuteExternalProjectSkipsCleanup(t *testing.T) {
	cleanupCalled := false
	result := Execute(context.Background(), ExecuteRequest{
		Plan:  Request{Compose: []string{"compose"}, Project: "leedevkit-dev", Tool: "go", Args: []string{"version"}, Service: "go"},
		Owned: false, CleanupBase: []string{"compose", "-p", "leedevkit-dev"},
		RunProcess: func(_ context.Context, _ process.Request) process.Result {
			return process.Result{ExitCode: 3, Err: errors.New("failed")}
		},
		RunCompose: func([]string) error { cleanupCalled = true; return nil },
	})
	if result.Process.ExitCode != 3 || cleanupCalled {
		t.Fatalf("result = %#v, cleanupCalled = %v", result, cleanupCalled)
	}
}

func TestExecuteCleansUpAfterProcessFailure(t *testing.T) {
	cleanupCalled := false
	result := Execute(context.Background(), ExecuteRequest{
		Plan:  Request{Compose: []string{"compose"}, Project: "leedevkit-test-failure", Tool: "go", Args: []string{"version"}, Service: "go"},
		Owned: true, CleanupBase: []string{"compose", "-p", "leedevkit-test-failure"},
		RunProcess: func(_ context.Context, _ process.Request) process.Result {
			return process.Result{ExitCode: 124, Err: errors.New("timeout")}
		},
		RunCompose: func([]string) error { cleanupCalled = true; return nil },
	})
	if result.Process.ExitCode != 124 || !cleanupCalled {
		t.Fatalf("result = %#v, cleanupCalled = %v", result, cleanupCalled)
	}
}
