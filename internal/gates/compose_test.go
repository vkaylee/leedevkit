package gates

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestComposeRunnerBuildsExecArgv(t *testing.T) {
	var got []string
	runner := ComposeRunner{
		Compose: []string{"docker", "compose"}, Project: "test", Service: "go", Workdir: "/workspace",
		Execute: func(_ context.Context, request process.Request) process.Result {
			got = request.Args
			return process.Result{ExitCode: 0}
		},
	}
	result := runner.Run(context.Background(), []string{"go", "test", "./..."}, nil)
	if result.ExitCode != 0 {
		t.Fatalf("result = %#v", result)
	}
	want := []string{"docker", "compose", "-p", "test", "exec", "-T", "-w", "/workspace", "go", "go", "test", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestComposeRunnerCapturesVersionOutput(t *testing.T) {
	runner := ComposeRunner{
		Compose: []string{"compose"}, Project: "test", Service: "go",
		Execute: func(_ context.Context, request process.Request) process.Result {
			if request.Stdout == nil {
				t.Fatal("stdout writer missing")
			}
			_, _ = request.Stdout.Write([]byte("tool 1.2.3\\n"))
			return process.Result{ExitCode: 0}
		},
	}
	result := runner.Run(context.Background(), []string{"tool", "--version"}, nil)
	if string(result.Stdout) != "tool 1.2.3\\n" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
}

func TestComposeRunnerBoundsCaptureButKeepsLog(t *testing.T) {
	var log bytes.Buffer
	data := bytes.Repeat([]byte("x"), 2<<20)
	runner := ComposeRunner{Compose: []string{"compose"}, Project: "test", Service: "go", Output: &log, Execute: func(_ context.Context, request process.Request) process.Result {
		_, _ = request.Stdout.Write(data)
		return process.Result{}
	}}
	result := runner.Run(context.Background(), []string{"test"}, nil)
	if result.ExitCode == 0 || len(result.Stdout) > 1<<20 || log.Len() != len(data) {
		t.Fatalf("exit=%d captured=%d log=%d", result.ExitCode, len(result.Stdout), log.Len())
	}
}

func TestComposeRunnerAllowsTruncationWhenStreamingOnly(t *testing.T) {
	var log bytes.Buffer
	data := bytes.Repeat([]byte("x"), 2<<20)
	runner := ComposeRunner{Output: &log, AllowTruncatedCapture: true, Execute: func(_ context.Context, request process.Request) process.Result {
		_, _ = request.Stdout.Write(data)
		return process.Result{}
	}}
	result := runner.Run(context.Background(), []string{"go", "test", "-json", "./..."}, nil)
	if result.ExitCode != 0 || len(result.Stdout) > 1<<20 || log.Len() != len(data) {
		t.Fatalf("exit=%d captured=%d log=%d", result.ExitCode, len(result.Stdout), log.Len())
	}
}

func TestComposeRunnerAddsEnvironment(t *testing.T) {
	var got []string
	runner := ComposeRunner{Compose: []string{"compose"}, Project: "test", Service: "go", Env: []string{"GOFLAGS=-race", "PRIVATE_VALUE=not-forwarded"}, Execute: func(_ context.Context, request process.Request) process.Result {
		got = request.Args
		return process.Result{}
	}}
	runner.Run(context.Background(), []string{"go", "vet", "./..."}, []string{"GOFLAGS"})
	want := []string{"compose", "-p", "test", "exec", "-T", "-e", "GOFLAGS", "go", "go", "vet", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}
