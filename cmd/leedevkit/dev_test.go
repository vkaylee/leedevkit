package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

func TestDevComposeArgsPreservesLifecycleBoundary(t *testing.T) {
	got, err := devComposeArgs([]string{"docker", "compose"}, "up")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "-p", "leedevkit-dev", "-f", "docker-compose.yml", "-f", ".compose/docker-compose.dev.yml", "up", "-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestDevComposeArgsSupportsReadOnlyOperations(t *testing.T) {
	for _, operation := range []string{"ps", "logs"} {
		args, err := devComposeArgs([]string{"compose"}, operation)
		if err != nil {
			t.Fatalf("%s args = %#v, err = %v", operation, args, err)
		}
		if operation == "logs" && args[len(args)-1] != "-f" {
			t.Fatalf("logs args = %#v", args)
		}
		if operation != "logs" && args[len(args)-1] != operation {
			t.Fatalf("%s args = %#v", operation, args)
		}
	}
}

func TestDevComposeArgsRejectsExecWithoutService(t *testing.T) {
	if _, err := devComposeArgsWithOptions([]string{"compose"}, "exec", "", nil); err == nil {
		t.Fatal("exec without service accepted")
	}
}

func TestRunDevPropagatesComposeResult(t *testing.T) {
	var got []string
	code := runDev("down", provider.Provider{Compose: []string{"compose"}}, "/project", nil, nil, func(_ context.Context, request process.Request) process.Result {
		got = request.Args
		return process.Result{ExitCode: 7}
	})
	if code != 7 || len(got) == 0 || got[len(got)-1] != "down" {
		t.Fatalf("code=%d args=%#v", code, got)
	}
}
