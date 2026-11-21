package debug

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if err != nil || got[len(got)-3] != "config" || got[len(got)-2] != "--format" || got[len(got)-1] != "json" {
		t.Fatalf("args = %#v, err = %v", got, err)
	}
}

func TestBuildDebugPSArgsTargetsOwnedDebugService(t *testing.T) {
	environment := Environment{ComposeProject: "fixture-project", ComposeFiles: []string{"compose.yml"}}
	got, err := BuildDebugPSArgs([]string{"compose"}, environment, "/tmp/debug.override.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "-p", "fixture-project", "-f", "compose.yml", "-f", "/tmp/debug.override.json", "ps", "-q", "leedevkit-debug"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
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

func TestComposeConfigValidationRejectsMissingServices(t *testing.T) {
	if err := ValidateComposeConfig(map[string]any{}, Environment{}); err == nil {
		t.Fatal("missing services accepted")
	}
}

func TestComposeConfigValidationRequiresApprovedNetworkAndService(t *testing.T) {
	environment := Environment{ComposeProject: "fixture-project", Networks: []string{"backend"}, Services: map[string]Service{"api": {ID: "api", Host: "api", Aliases: []string{"api"}}}}
	config := map[string]any{
		"services": map[string]any{"api": map[string]any{}},
		"networks": map[string]any{"backend": map[string]any{"name": "fixture-project_backend"}},
	}
	if err := ValidateComposeConfig(config, environment); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchRunsValidationBeforeDebugRun(t *testing.T) {
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ComposeFiles: []string{"compose.yml"}, Profiles: []string{"debug"}, Networks: []string{"backend"}, ProjectRoot: "/repo", SourcePath: "/repo/leedevkit.toml", Services: map[string]Service{"api": {ID: "api", Host: "api", Aliases: []string{"api"}}}}
	calls := [][]string{}
	runner := func(_ context.Context, args []string) error {
		calls = append(calls, args)
		return nil
	}
	if err := LaunchValidated(context.Background(), environment, []string{"compose"}, []string{"docker"}, runner, []byte(`{"services":{"api":{}},"networks":{"backend":{}}}`), []byte(`[{"Labels":{"com.docker.compose.project":"fixture-project"}}]`)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || !contains(calls[0], "leedevkit-debug") || !contains(calls[1], "leedevkit-debug") || !contains(calls[2], "ps") {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestLaunchRunsValidationBeforeDebugRunRejectsOwnerMismatch(t *testing.T) {
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ProjectRoot: "/repo", SourcePath: "/repo/leedevkit.toml", Networks: []string{"backend"}}
	calls := 0
	err := LaunchValidated(context.Background(), environment, []string{"compose"}, []string{"docker"}, func(context.Context, []string) error { calls++; return nil }, []byte(`{"services":{},"networks":{"backend":{}}}`), []byte(`[{"Labels":{"com.docker.compose.project":"other"}}]`))
	if err == nil || calls != 0 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}
}

func TestLaunchRunsComposeAndCleansOwnedService(t *testing.T) {
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ComposeFiles: []string{"compose.yml"}, Profiles: []string{"debug"}, ProjectRoot: "/repo", SourcePath: "/repo/leedevkit.toml"}
	calls := [][]string{}
	result := Launch(context.Background(), environment, []string{"compose"}, func(_ context.Context, args []string) error {
		calls = append(calls, args)
		return nil
	})
	if result != nil || len(calls) != 3 || !contains(calls[0], "leedevkit-debug") || !contains(calls[1], "leedevkit-debug") || !contains(calls[2], "ps") {
		t.Fatalf("result = %v, calls = %#v", result, calls)
	}
}

func TestLaunchCleansUpAfterContextCancellation(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(manifest, []byte("debug = {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ProjectRoot: root, SourcePath: manifest}
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	result := Launch(ctx, environment, []string{"compose"}, func(callCtx context.Context, args []string) error {
		calls++
		if calls == 1 {
			cancel()
			return context.Canceled
		}
		if callCtx.Err() != nil {
			t.Fatalf("cleanup context canceled: %v", callCtx.Err())
		}
		if calls == 2 && (!contains(args, "rm") || contains(args, "down")) {
			t.Fatalf("unsafe cleanup args: %#v", args)
		}
		if calls == 3 && (!contains(args, "ps") || !contains(args, "-q")) {
			t.Fatalf("cleanup verification args: %#v", args)
		}
		return nil
	})
	if result == nil || calls != 3 {
		t.Fatalf("result = %v, calls = %d", result, calls)
	}
}

func TestLaunchWithValidationRejectsResidualDebugContainer(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(manifest, []byte("debug = {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ProjectRoot: root, SourcePath: manifest, Networks: []string{"backend"}, Services: map[string]Service{"api": {ID: "api", Host: "api"}}}
	output := func(_ context.Context, args []string) ([]byte, error) {
		switch {
		case contains(args, "config"):
			return []byte(`{"services":{"api":{}},"networks":{"backend":{}}}`), nil
		case contains(args, "inspect"):
			return []byte(`[{"Labels":{"com.docker.compose.project":"fixture-project"}}]`), nil
		case contains(args, "ps"):
			return []byte("residual-container\n"), nil
		default:
			return nil, nil
		}
	}
	err := LaunchWithValidation(context.Background(), environment, []string{"compose"}, []string{"docker"}, func(context.Context, []string) error { return nil }, output)
	if err == nil || !strings.Contains(err.Error(), "residual") {
		t.Fatalf("err = %v, want residual container error", err)
	}
}

func TestLaunchWithValidationInspectsEveryResolvedNetwork(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(manifest, []byte("debug = {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ProjectRoot: root, SourcePath: manifest, Networks: []string{"backend", "frontend"}, Services: map[string]Service{"api": {ID: "api", Host: "api"}}}
	var inspected []string
	output := func(_ context.Context, args []string) ([]byte, error) {
		if contains(args, "config") {
			return []byte(`{"services":{"api":{}},"networks":{"backend":{},"frontend":{"name":"shared_frontend"}}}`), nil
		}
		if contains(args, "ps") {
			return nil, nil
		}
		inspected = append(inspected, args[len(args)-1])
		return []byte(`[{"Labels":{"com.docker.compose.project":"fixture-project"}}]`), nil
	}
	if err := LaunchWithValidation(context.Background(), environment, []string{"compose"}, []string{"docker"}, func(context.Context, []string) error { return nil }, output); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inspected, []string{"fixture-project_backend", "shared_frontend"}) {
		t.Fatalf("inspected = %#v", inspected)
	}
}

func TestLaunchValidatedUsesResolvedNetworkNamesInOverride(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(manifest, []byte("debug = {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := Environment{Name: "fixture", ComposeProject: "fixture-project", ProjectRoot: root, SourcePath: manifest, Networks: []string{"backend"}}
	calls := [][]string{}
	var overrideData []byte
	if err := LaunchValidated(context.Background(), environment, []string{"compose"}, []string{"docker"}, func(_ context.Context, args []string) error {
		calls = append(calls, args)
		if len(calls) == 1 {
			for _, arg := range args {
				if strings.HasSuffix(arg, "debug-compose.override.json") {
					var err error
					overrideData, err = os.ReadFile(arg)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		return nil
	}, []byte(`{"services":{"api":{}},"networks":{"backend":{"name":"shared_backend"}}}`), []byte(`[{"Labels":{"com.docker.compose.project":"fixture-project"}}]`)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || !strings.Contains(string(overrideData), "shared_backend") {
		t.Fatalf("calls = %#v, override = %s", calls, overrideData)
	}
}
