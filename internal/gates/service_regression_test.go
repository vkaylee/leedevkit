package gates

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestComposeRunnerOverridesServiceWithoutMutatingOriginal(t *testing.T) {
	original := ComposeRunner{Compose: []string{"docker", "compose"}, Project: "project", Service: "default", Workdir: "/workspace", Env: []string{"SECRET=value"}}
	var got []string
	original.Execute = func(_ context.Context, request process.Request) process.Result {
		got = append([]string(nil), request.Args...)
		fmt.Fprint(request.Stdout, "output")
		fmt.Fprint(request.Stderr, "diagnostic")
		return process.Result{ExitCode: 3}
	}
	result := original.WithService("backend", "/workspace/backend").Run(context.Background(), []string{"go", "test"}, []string{"GOFLAGS"})
	want := []string{"docker", "compose", "-p", "project", "exec", "-T", "-w", "/workspace/backend", "-e", "GOFLAGS", "backend", "go", "test"}
	if !reflect.DeepEqual(got, want) || result.ExitCode != 3 || string(result.Stdout) != "output" || string(result.Stderr) != "diagnostic" {
		t.Fatalf("args=%q result=%+v", got, result)
	}
	if original.Service != "default" || original.Workdir != "/workspace" {
		t.Fatalf("original mutated: %+v", original)
	}
}
