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
