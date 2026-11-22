package testplan

import (
	"context"
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/process"
)

func TestGatePhaseExecutorMapsSelectedPhasesToCompose(t *testing.T) {
	for _, tc := range []struct{ phase, gatePhase string }{
		{"Integration Tests", "integration"}, {"Linting", "lint"}, {"Coverage", "coverage"},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			var gotArgs, gotEnv []string
			executor := GatePhaseExecutor{
				Definitions: []gates.Gate{{Name: "fixture", Phase: tc.gatePhase, Command: []string{"check"}, Environment: []string{"GOFLAGS"}}},
				ComposeRun: func(_ context.Context, args, env []string) process.Result {
					gotArgs, gotEnv = args, env
					return process.Result{}
				},
			}
			result := executor.ExecuteContext(context.Background(), Phase{Name: tc.phase, Status: "selected"})
			if result.Status != "passed" || !reflect.DeepEqual(gotArgs, []string{"check"}) || !reflect.DeepEqual(gotEnv, []string{"GOFLAGS"}) {
				t.Fatalf("result=%+v args=%v env=%v", result, gotArgs, gotEnv)
			}
		})
	}
}
