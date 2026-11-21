package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestReadinessWaitsForHealthyServices(t *testing.T) {
	calls := 0
	checker := Readiness{Run: func(_ context.Context, request process.Request) process.Result {
		calls++
		if calls < 2 {
			return process.Result{ExitCode: 1}
		}
		return process.Result{ExitCode: 0}
	}}
	if err := checker.Wait(context.Background(), []string{"docker", "compose", "ps", "--status", "running"}, []string{"go"}, 250*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestReadinessTimesOut(t *testing.T) {
	checker := Readiness{Run: func(context.Context, process.Request) process.Result { return process.Result{ExitCode: 1} }}
	if err := checker.Wait(context.Background(), []string{"compose", "ps"}, []string{"go"}, 10*time.Millisecond); err == nil {
		t.Fatal("expected readiness timeout")
	}
}
