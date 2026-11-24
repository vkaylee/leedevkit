package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestReadinessWaitsForHealthyServices(t *testing.T) {
	calls := 0
	checker := Readiness{Dir: "/project", Env: []string{"PROJECT_ROOT=/project"}, Run: func(_ context.Context, request process.Request) process.Result {
		calls++
		if request.Dir != "/project" || len(request.Env) != 1 {
			t.Fatalf("request = %#v", request)
		}
		if calls < 2 {
			return process.Result{ExitCode: 0, Stdout: []byte(`[]`)}
		}
		return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"project-go-1","State":"running","Health":"healthy"}]`)}
	}}
	if err := checker.Wait(context.Background(), []string{"docker", "compose", "ps", "--status", "running"}, []string{"go"}, 250*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestReadinessRejectsUnhealthyService(t *testing.T) {
	checker := Readiness{Run: func(context.Context, process.Request) process.Result {
		return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"api-1","State":"running","Health":"unhealthy"}]`)}
	}}
	if err := checker.Wait(context.Background(), []string{"compose", "ps"}, []string{"api"}, 10*time.Millisecond); err == nil {
		t.Fatal("unhealthy service reported ready")
	}
}

func TestReadinessTimesOut(t *testing.T) {
	checker := Readiness{Run: func(context.Context, process.Request) process.Result {
		return process.Result{ExitCode: 0, Stdout: []byte(`[]`)}
	}}
	if err := checker.Wait(context.Background(), []string{"compose", "ps"}, []string{"go"}, 10*time.Millisecond); err == nil {
		t.Fatal("expected readiness timeout")
	}
}
