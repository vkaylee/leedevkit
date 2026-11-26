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
	var gotEnv []string
	runner := ComposeRunner{Compose: []string{"compose"}, Project: "test", Service: "go", Env: []string{"GOFLAGS=-race", "PRIVATE_VALUE=not-forwarded"}, Execute: func(_ context.Context, request process.Request) process.Result {
		got = request.Args
		gotEnv = request.Env
		return process.Result{}
	}}
	runner.Run(context.Background(), []string{"go", "vet", "./..."}, []string{"GOFLAGS", "APP_DB_INTEGRATION=1", "API_TOKEN=supersecret"})
	want := []string{"compose", "-p", "test", "exec", "-T", "-e", "GOFLAGS", "-e", "APP_DB_INTEGRATION", "-e", "API_TOKEN", "go", "go", "vet", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
	for _, arg := range got {
		if arg == "supersecret" || arg == "API_TOKEN=supersecret" || arg == "APP_DB_INTEGRATION=1" {
			t.Fatalf("argv must never contain env values or secrets: %#v", got)
		}
	}
	foundFlag := false
	foundSecret := false
	for _, entry := range gotEnv {
		if entry == "APP_DB_INTEGRATION=1" {
			foundFlag = true
		}
		if entry == "API_TOKEN=supersecret" {
			foundSecret = true
		}
	}
	if !foundFlag || !foundSecret {
		t.Fatalf("expected static env pairs in child process Env, got %#v", gotEnv)
	}
}
