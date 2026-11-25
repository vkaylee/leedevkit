package lifecycle

import (
	"context"
	"errors"
	"testing"
)

func TestExecutorRunsUpAndDownAroundWork(t *testing.T) {
	calls := []string{}
	executor := Executor{
		UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
		Execute: func(_ context.Context, args []string) error { calls = append(calls, args[1]); return nil },
	}
	if err := executor.Run(context.Background(), func() error { calls = append(calls, "work"); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[0] != "up" || calls[1] != "work" || calls[2] != "down" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestExecutorBlocksWorkWhenUpFailsAndStillAttemptsDown(t *testing.T) {
	calls := []string{}
	executor := Executor{
		UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
		Execute: func(_ context.Context, args []string) error {
			calls = append(calls, args[1])
			if args[1] == "up" {
				return errors.New("startup")
			}
			return nil
		},
	}
	if err := executor.Run(context.Background(), func() error { calls = append(calls, "work"); return nil }); err == nil {
		t.Fatal("expected startup error")
	}
	if len(calls) != 2 || calls[0] != "up" || calls[1] != "down" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestExecutorUsesLiveContextForTeardownAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	teardownCanceled := false
	executor := Executor{
		UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
		Execute: func(callCtx context.Context, args []string) error {
			if args[1] == "down" {
				calls++
				if callCtx.Err() != nil {
					teardownCanceled = true
					return callCtx.Err()
				}
			}
			return nil
		},
	}
	result := executor.Run(ctx, func() error {
		cancel()
		return context.Canceled
	})
	if result == nil || calls != 1 || teardownCanceled {
		t.Fatalf("result = %v, teardown calls = %d, canceled = %v", result, calls, teardownCanceled)
	}
}

func TestExecutorReturnsWorkErrorAndRunsDown(t *testing.T) {
	calls := []string{}
	workErr := errors.New("gate failed")
	executor := Executor{
		UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
		Execute: func(_ context.Context, args []string) error { calls = append(calls, args[1]); return nil },
	}
	if err := executor.Run(context.Background(), func() error { calls = append(calls, "work"); return workErr }); !errors.Is(err, workErr) {
		t.Fatalf("error = %v", err)
	}
	if len(calls) != 3 || calls[2] != "down" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestExecutorUsesLiveContextForDownAfterCanceledStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	downCalls := 0
	var downErr error
	executor := Executor{
		UpArgs: []string{"compose", "up"}, DownArgs: []string{"compose", "down"},
		Execute: func(callCtx context.Context, args []string) error {
			if args[1] == "up" {
				cancel()
				return context.Canceled
			}
			downCalls++
			downErr = callCtx.Err()
			return nil
		},
	}
	if err := executor.Run(ctx, func() error { t.Fatal("work must not run"); return nil }); err == nil {
		t.Fatal("expected startup error")
	}
	if downCalls != 1 || downErr != nil {
		t.Fatalf("downCalls=%d downErr=%v", downCalls, downErr)
	}
}
