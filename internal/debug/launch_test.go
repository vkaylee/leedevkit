package debug

import (
	"context"
	"reflect"
	"testing"
)

func TestComposeBaseArgsIncludesProjectFilesAndProfiles(t *testing.T) {
	environment := Environment{ComposeProject: "fixture-project", ComposeFiles: []string{"compose.yml", ".compose.debug.yml"}, Profiles: []string{"debug"}}
	got, err := ComposeBaseArgs([]string{"docker", "compose"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "-p", "fixture-project", "-f", "compose.yml", "-f", ".compose.debug.yml", "--profile", "debug"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestBuildComposeConfigArgs(t *testing.T) {
	environment := Environment{ComposeProject: "fixture-project", ComposeFiles: []string{"compose.yml"}, Profiles: []string{"debug"}}
	got, err := BuildComposeConfigArgs([]string{"compose"}, environment)
	if err != nil || got[len(got)-1] != "config" {
		t.Fatalf("args = %#v, err = %v", got, err)
	}
}

func TestBuildDebugRunArgsUsesReadOnlyContainer(t *testing.T) {
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ComposeFiles: []string{"compose.yml"}, Profiles: []string{"debug"}, ProjectRoot: "/repo", SourcePath: "/repo/leedevkit.toml"}
	got, err := BuildDebugRunArgs([]string{"compose"}, environment, "/tmp/debug.override.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"--rm", "-T", "--no-deps", "--build", "leedevkit-debug", "--environment", "fixture", "--manifest", "/workspace/leedevkit.toml", "--project-root", "/workspace"} {
		if !contains(got, value) {
			t.Fatalf("args missing %q: %#v", value, got)
		}
	}
}

func TestLaunchRunsComposeAndCleansOwnedService(t *testing.T) {
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ComposeFiles: []string{"compose.yml"}, Profiles: []string{"debug"}, ProjectRoot: "/repo", SourcePath: "/repo/leedevkit.toml"}
	calls := [][]string{}
	result := Launch(context.Background(), environment, []string{"compose"}, func(_ context.Context, args []string) error {
		calls = append(calls, args)
		return nil
	})
	if result != nil || len(calls) != 2 || !contains(calls[0], "leedevkit-debug") || !contains(calls[1], "leedevkit-debug") {
		t.Fatalf("result = %v, calls = %#v", result, calls)
	}
}
