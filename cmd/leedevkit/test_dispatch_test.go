package main

import (
	"context"
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

func TestInfraTargetRecognizesOnlyInfraAlias(t *testing.T) {
	project := config.Project{Targets: map[string][]string{"all": {"infra"}}}
	if !isInfraTarget(project, "infra") || !isInfraTarget(project, "all") {
		t.Fatal("infra and its all alias must select infrastructure checks")
	}
	project.Targets["all"] = []string{"infra", "web"}
	if isInfraTarget(project, "all") {
		t.Fatal("mixed targets must not silently skip other checks")
	}
}

func TestInfraRunRequiresEveryCheckAndPropagatesFailure(t *testing.T) {
	var commands [][]string
	runner := func(_ context.Context, request process.Request) process.Result {
		commands = append(commands, request.Args)
		if len(commands) == 3 {
			return process.Result{ExitCode: 7}
		}
		return process.Result{}
	}
	result := runInfra("/fixture", testplan.Flags{Target: "infra"}, runner, nil)
	if result.Status != "failed" || result.ExitCode != 7 || result.FullRegression {
		t.Fatalf("result = %#v", result)
	}
	if len(commands) != 9 {
		t.Fatalf("ran %d checks, want 9", len(commands))
	}
}

func TestInfraRejectsUnrecognizedFlags(t *testing.T) {
	for _, flag := range []string{"--typo", "--dry-run", "--json-stream", "--unit-only", "--skip-build"} {
		if err := validateInfraFlags([]string{flag}); err == nil {
			t.Errorf("accepted %s", flag)
		}
	}
	for _, flag := range []string{"--json", "--quiet"} {
		if err := validateInfraFlags([]string{flag}); err != nil {
			t.Errorf("rejected %s: %v", flag, err)
		}
	}
}

func TestInfraUsesReadOnlyDockerSourceMount(t *testing.T) {
	runInfra("/fixture", testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		if len(request.Args) > 0 && request.Args[0] == "docker" {
			found := false
			for _, arg := range request.Args {
				found = found || arg == "/fixture:/workspace:ro"
			}
			if !found {
				t.Errorf("docker source mount is not read-only: %v", request.Args)
			}
		}
		return process.Result{}
	}, nil)
}

func TestInfraRejectsOtherProjects(t *testing.T) {
	if err := validateInfraProject(t.TempDir()); err == nil {
		t.Fatal("infra must reject projects without the repository checks")
	}
}

func TestInfraRunDoesNotClaimFullRegressionForCIChecks(t *testing.T) {
	result := runInfra("/fixture", testplan.Flags{Target: "infra"}, func(context.Context, process.Request) process.Result {
		return process.Result{}
	}, nil)
	if result.Status != "partial" || result.FullRegression || result.ExitCode == 0 {
		t.Fatalf("result = %#v; missing legacy gates must not be reported as full regression", result)
	}
}

func TestShouldUseGoTestUsesGoForInfra(t *testing.T) {
	if !shouldUseGoTest(config.Project{}, "infra") {
		t.Fatal("infra dispatch should use Go")
	}
}

func TestShouldUseGoTestRequiresConfiguredGoTarget(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{"backend": {Language: "go"}}, Targets: map[string][]string{"all": {"go"}}}
	if !shouldUseGoTest(project, "go") {
		t.Fatal("configured Go target should use Go")
	}
	if !shouldUseGoTest(project, "all") {
		t.Fatal("Go-only all target should use Go")
	}
}

func TestShouldUseGoTestRejectsUnsupportedTargets(t *testing.T) {
	goProject := config.Project{Services: map[string]config.Service{"backend": {Language: "go"}}, Targets: map[string][]string{"all": {"go", "web"}}}
	if shouldUseGoTest(goProject, "all") {
		t.Fatal("mixed target must not claim Go parity")
	}
}
