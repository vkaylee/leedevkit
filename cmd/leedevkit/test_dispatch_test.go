package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
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
	result := runInfra(t.TempDir(), testplan.Flags{Target: "infra"}, runner, nil)
	if result.Status != "failed" || result.ExitCode != 7 || result.FullRegression {
		t.Fatalf("result = %#v", result)
	}
	if len(commands) != 12 {
		t.Fatalf("ran %d checks, want 12", len(commands))
	}
}

func TestInfraIncludesGoQualityGates(t *testing.T) {
	var commands []string
	runInfra(t.TempDir(), testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		commands = append(commands, strings.Join(request.Args, " "))
		return process.Result{}
	}, nil)
	joined := strings.Join(commands, "\n")
	for _, check := range []string{"gofmt", "go vet", "go test -cover"} {
		if !strings.Contains(joined, check) {
			t.Errorf("missing %s gate", check)
		}
	}
}

func TestInfraRejectsUnrecognizedFlags(t *testing.T) {
	for _, flag := range []string{"--typo", "--dry-run", "--unit-only", "--skip-build"} {
		if err := validateInfraFlags([]string{flag}); err == nil {
			t.Errorf("accepted %s", flag)
		}
	}
	for _, flag := range []string{"--json", "--quiet", "--json-stream"} {
		if err := validateInfraFlags([]string{flag}); err != nil {
			t.Errorf("rejected %s: %v", flag, err)
		}
	}
}

func TestInfraUsesReadOnlyDockerSourceMount(t *testing.T) {
	root := t.TempDir()
	runInfra(root, testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		if len(request.Args) > 0 && request.Args[0] == "docker" {
			found := false
			for _, arg := range request.Args {
				found = found || arg == root+":/workspace:ro"
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

func TestInfraRunPassesWhenConfiguredChecksPass(t *testing.T) {
	result := runInfra(t.TempDir(), testplan.Flags{Target: "infra"}, func(context.Context, process.Request) process.Result {
		return process.Result{}
	}, nil)
	if result.Status != "passed" || !result.FullRegression || result.ExitCode != 0 {
		t.Fatalf("result = %#v; configured checks passed", result)
	}
}

func TestGoTestRejectsInvalidPatternBeforeRunningAnything(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[services.backend]\nlang = 'go'\n[targets]\nall = ['go']\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := run([]string{"test", "all", "--pattern", "["}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--pattern") {
		t.Fatalf("exit = %d, stderr = %q; want unsupported pattern error", code, stderr.String())
	}
}

func TestGoTestRejectsEmptyPatternAndUnknownFlags(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[services.backend]\nlang = 'go'\n[targets]\nall = ['go']\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, tc := range []struct {
		args []string
		want string
	}{{[]string{"test", "all", "--pattern", ""}, "--pattern"}, {[]string{"test", "all", "--typo"}, "--typo"}} {
		var stdout, stderr bytes.Buffer
		code := run(tc.args, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("args = %q, exit = %d, stderr = %q; want explicit rejection", tc.args, code, stderr.String())
		}
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
