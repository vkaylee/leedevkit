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
