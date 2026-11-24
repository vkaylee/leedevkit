package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/provider"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

func TestWebPhaseCommands(t *testing.T) {
	for phase, want := range map[string][][]string{
		"Unit Tests":        {{"bun", "run", "test"}},
		"Integration Tests": {{"bun", "run", "test:e2e"}},
		"Linting":           {{"bun", "run", "lint"}, {"bun", "run", "type-check"}, {"bun", "run", "check-i18n"}},
	} {
		got := webPhaseCommands(phase)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("webPhaseCommands(%q) = %v, want %v", phase, got, want)
		}
	}
	if got := webPhaseCommands("Unknown"); got != nil {
		t.Fatalf("unknown phase commands = %v", got)
	}
}

func TestPhaseDependencyKeyPrefersDirectServiceOverSharedAlias(t *testing.T) {
	project := config.Project{
		Services:     map[string]config.Service{"web": {Language: "typescript"}, "admin": {Language: "typescript"}},
		Targets:      map[string][]string{"all": {"web", "admin"}},
		Dependencies: map[string][]string{"unit-web": {"web"}, "unit-admin": {"admin"}, "unit-all": {"web"}},
	}
	if got := phaseDependencyKey(project, "Unit Tests", "admin"); got != "unit-admin" {
		t.Fatalf("admin dependency key = %q", got)
	}
}

func TestLeeAttendManifestDefaultsAndPhases(t *testing.T) {
	project := leeAttendFixture(t)
	if hasPhaseGate(project.Gates, "integration") {
		t.Fatal("expected LeeAttend config to omit Go integration gates")
	}
	if got := phaseDependencyKey(project, "Integration Tests", "api"); got != "int-api" {
		t.Fatalf("Go integration key = %q", got)
	}
	if got := phaseDependencyKey(project, "Integration Tests", "webdashboard"); got != "e2e-web" {
		t.Fatalf("web e2e key = %q; service=%#v deps=%#v", got, project.Services["webdashboard"], project.Dependencies)
	}
	if _, _, _, err := buildWebLifecyclePlan("/fixture", "fixture", provider.Provider{Name: "fixture", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}, project, "webdashboard", "Integration Tests", testplan.Flags{Build: true}, false); err != nil {
		t.Fatal(err)
	}
}

func leeAttendFixture(t *testing.T) config.Project {
	t.Helper()
	root := t.TempDir()
	manifest := `[services.apiserver]
lang = "go"
workdir = "apiserver"
[services.api]
lang = "go"
workdir = "apiserver"
[services.webdashboard]
lang = "typescript"
dockerfile = "./webdashboard"
[targets]
api = ["api"]
web = ["webdashboard"]
all = ["api", "webdashboard"]
[test.dependencies]
lint-api = ["apiserver"]
unit-api = ["apiserver"]
int-api = ["apiserver"]
lint-go = ["apiserver"]
unit-go = ["apiserver"]
int-go = ["apiserver"]
lint-web = ["bun_init", "webdashboard"]
unit-web = ["bun_init", "webdashboard"]
e2e-web = ["bun_init", "webdashboard"]
`
	path := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func hasPhaseGate(definitions []gates.Gate, phase string) bool {
	for _, gate := range definitions {
		if gate.Phase == phase {
			return true
		}
	}
	return false
}

func TestLeeAttendConfigLoadRunsThroughAllAndE2EPlan(t *testing.T) {
	project := leeAttendFixture(t)
	services, err := configuredTargets(project, "all")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(services, []string{"api", "webdashboard"}) {
		t.Fatalf("services = %v", services)
	}
	goPhase := runGoPhase(t.TempDir(), "fixture", provider.Provider{}, project, "api", "Integration Tests", testplan.Flags{}, true, io.Discard, io.Discard)
	if goPhase.Status != "skipped" {
		t.Fatalf("missing Go integration gate result = %#v", goPhase)
	}
	webPlan, _, _, err := buildWebLifecyclePlan("/fixture", "fixture", provider.Provider{Name: "fixture", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}, project, "webdashboard", "Integration Tests", testplan.Flags{Build: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	up, err := webPlan.Up()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(up, " "), "e2e-web") {
		t.Fatalf("web e2e not selected: %v", up)
	}
}

func TestLeeAttendFixtureAliasAndDependencies(t *testing.T) {
	project := config.Project{
		Services: map[string]config.Service{
			"apiserver":    {Language: "go", Workdir: "apiserver"},
			"api":          {Language: "go", Workdir: "apiserver"},
			"webdashboard": {Language: "typescript", Dockerfile: "./webdashboard"},
		},
		Targets: map[string][]string{"api": {"api"}, "web": {"webdashboard"}, "all": {"api", "webdashboard"}},
		Dependencies: map[string][]string{
			"lint-api": {"apiserver"}, "unit-api": {"apiserver"}, "int-api": {"apiserver"},
			"lint-web": {"bun_init", "webdashboard"}, "unit-web": {"bun_init", "webdashboard"}, "e2e-web": {"bun_init", "webdashboard"},
		},
	}
	targets, err := configuredTargets(project, "all")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(targets, []string{"api", "webdashboard"}) {
		t.Fatalf("all targets = %v", targets)
	}
	if got, err := configuredTargets(project, "web"); err != nil || !reflect.DeepEqual(got, []string{"webdashboard"}) {
		t.Fatalf("web target = %v, err=%v", got, err)
	}
	for phase, key := range map[string]string{"Unit Tests": "unit-web", "Integration Tests": "e2e-web", "Linting": "lint-web"} {
		deps, runner, err := phaseService(project, "webdashboard", phase)
		if err != nil {
			t.Fatal(err)
		}
		if key != phaseDependencyKey(project, phase, "webdashboard") || runner != "webdashboard" || !reflect.DeepEqual(deps, []string{"bun_init", "webdashboard"}) {
			t.Errorf("%s: key=%s runner=%s dependencies=%v", phase, phaseDependencyKey(project, phase, "webdashboard"), runner, deps)
		}
	}
	plan, service, workdir, err := buildWebLifecyclePlan("/fixture", "fixture", provider.Provider{Name: "fixture", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}, project, "webdashboard", "Linting", testplan.Flags{Build: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	args, err := plan.Up()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"--profile lint-web", "--build", "bun_init", "webdashboard"} {
		if !strings.Contains(joined, want) {
			t.Errorf("up args %q missing %q", joined, want)
		}
	}
	if service != "webdashboard" || workdir != "/workspace/webdashboard" {
		t.Fatalf("service/workdir = %q/%q", service, workdir)
	}
}

func TestGoGateFailureKeepsOriginalExitCode(t *testing.T) {
	gate := gates.PhaseResult{Status: "failed", ExitCode: 7, Diagnostic: "gate failed"}
	status, code := phaseFailureStatus(gate, fmt.Errorf("lifecycle gate failed"))
	if status != "failed" || code != 7 {
		t.Fatalf("status=%s code=%d", status, code)
	}
}

func TestGoPhaseFailsWhenComposeExecFails(t *testing.T) {
	phase, err := goPhaseWithCompose(context.Background(), func(_ context.Context, args []string) error {
		if strings.Contains(strings.Join(args, " "), "exec") {
			return fmt.Errorf("exec failed")
		}
		return nil
	}, []string{"compose", "up"}, []string{"compose", "down"}, func(context.Context) gates.PhaseResult {
		return gates.PhaseResult{Status: "failed", ExitCode: 7, Diagnostic: "exec failed"}
	})
	if err == nil || phase.Status != "failed" {
		t.Fatalf("phase=%#v err=%v", phase, err)
	}
}

func TestRunGoPhaseRequiresConfiguredGates(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{"api": {Language: "go"}}, Dependencies: map[string][]string{"unit-api": {"apiserver"}}}
	got := runGoPhase(t.TempDir(), "fixture", provider.Provider{Name: "fixture", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}, project, "api", "Unit Tests", testplan.Flags{}, true, io.Discard, io.Discard)
	if got.Status != "blocked" || !strings.Contains(got.Diagnostic, "no configured Go gates") {
		t.Fatalf("result = %#v", got)
	}
}

func TestLifecycleExecutesWorkAfterUpAndSurfacesDownFailure(t *testing.T) {
	var calls []string
	phase, err := goPhaseWithCompose(context.Background(), func(_ context.Context, args []string) error {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		if strings.Contains(call, "down") {
			return fmt.Errorf("down failed")
		}
		if strings.Contains(call, "up") || strings.Contains(call, "exec") {
			return nil
		}
		return nil
	}, []string{"compose", "up"}, []string{"compose", "down"}, func(context.Context) gates.PhaseResult {
		calls = append(calls, "gate")
		return gates.PhaseResult{Status: "passed"}
	})
	status, code := lifecycleFailureStatus(err)
	if err == nil || phase.Status != "passed" || status != "failed" || code != 1 {
		t.Fatalf("phase=%#v error=%v status=%s code=%d", phase, err, status, code)
	}
	if !reflect.DeepEqual(calls, []string{"compose up", "gate", "compose down"}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestGoPhaseWaitsForReadinessBeforeGate(t *testing.T) {
	var calls []string
	_, err := goPhaseWithCompose(context.Background(), func(_ context.Context, args []string) error {
		calls = append(calls, strings.Join(args, " "))
		return nil
	}, []string{"compose", "up"}, []string{"compose", "down"}, func(context.Context) gates.PhaseResult {
		calls = append(calls, "gate")
		return gates.PhaseResult{Status: "passed"}
	}, func(context.Context) error { calls = append(calls, "ready"); return nil })
	if err != nil || !reflect.DeepEqual(calls, []string{"compose up", "ready", "gate", "compose down"}) {
		t.Fatalf("calls=%v err=%v", calls, err)
	}
}

func TestLifecycleStartupFailurePreventsGateExec(t *testing.T) {
	var calls []string
	_, err := goPhaseWithCompose(context.Background(), func(_ context.Context, args []string) error {
		calls = append(calls, strings.Join(args, " "))
		if strings.HasSuffix(strings.Join(args, " "), "up") {
			return fmt.Errorf("up failed")
		}
		return nil
	}, []string{"compose", "up"}, []string{"compose", "down"}, func(context.Context) gates.PhaseResult {
		calls = append(calls, "gate")
		return gates.PhaseResult{Status: "passed"}
	})
	if err == nil || len(calls) != 2 || calls[0] != "compose up" || calls[1] != "compose down" {
		t.Fatalf("calls=%v err=%v", calls, err)
	}
}

func TestJSONFlagWritesSummaryWithQuietOutput(t *testing.T) {
	root := t.TempDir()
	manifest := "[services.webdashboard]\nlang='typescript'\ndockerfile='./webdashboard'\n[targets]\nweb=['webdashboard']\nall=['webdashboard']\n[test.dependencies]\nunit-web=['bun_init','webdashboard']\nint-api=['apiserver']\nlint-web=['bun_init','webdashboard']\ne2e-web=['bun_init','webdashboard']\n"
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	t.Setenv("CONTAINER_PROVIDER", "docker")
	var stdout, stderr bytes.Buffer
	code := run([]string{"test", "web", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema_version":1`) {
		t.Fatalf("JSON summary missing: %q", stdout.String())
	}
}

func TestWebDispatchKeepsJSONStreamClean(t *testing.T) {
	// Compose command output is directed to logs in the runner; JSON stream writer is separate.
	var stream bytes.Buffer
	result := testplan.Run(testplan.Flags{Target: "all", JSONStream: true}, testplan.ExecutorFunc(func(phase testplan.Phase) testplan.PhaseResult { return testplan.PhaseResult{Status: "passed"} }), &stream)
	if result.ExitCode != 0 || !strings.Contains(stream.String(), `"event":"run_started"`) {
		t.Fatalf("result=%#v stream=%q", result, stream.String())
	}
	if strings.Contains(stream.String(), "stdout noise") {
		t.Fatalf("non-JSON command output leaked: %q", stream.String())
	}
}

func hasArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func TestBuildOnceStartsWithBuildThenSkipsBuild(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{"api": {Language: "go"}}, Dependencies: map[string][]string{"unit-api": {"apiserver"}, "lint-api": {"apiserver"}}}
	providerConfig := provider.Provider{Name: "fixture", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}
	flags := testplan.Flags{BuildPolicy: "once", Build: true}
	first := buildGoLifecyclePlanForPhase("/fixture", "fixture", providerConfig, project, "api", "Unit Tests", flags, false)
	flags.Build = false
	second := buildGoLifecyclePlanForPhase("/fixture", "fixture", providerConfig, project, "api", "Linting", flags, false)
	firstArgs, err := first.Up()
	if err != nil {
		t.Fatal(err)
	}
	secondArgs, err := second.Up()
	if err != nil {
		t.Fatal(err)
	}
	if !hasArg(firstArgs, "--build") || !hasArg(secondArgs, "--no-build") {
		t.Fatalf("first=%v second=%v", firstArgs, secondArgs)
	}
}

func TestWebLifecycleBuildPolicyUsesNoBuildAfterFirst(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{"webdashboard": {Language: "typescript"}}, Targets: map[string][]string{"web": {"webdashboard"}}, Dependencies: map[string][]string{"unit-web": {"bun_init", "webdashboard"}}}
	providerConfig := provider.Provider{Name: "fixture", Compose: []string{"compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}
	built, _, _, err := buildWebLifecyclePlan("/fixture", "fixture", providerConfig, project, "webdashboard", "Unit Tests", testplan.Flags{Build: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	flags := testplan.Flags{Build: false}
	noBuild, _, _, err := buildWebLifecyclePlan("/fixture", "fixture", providerConfig, project, "webdashboard", "Unit Tests", flags, false)
	if err != nil {
		t.Fatal(err)
	}
	builtArgs, err := built.Up()
	if err != nil {
		t.Fatal(err)
	}
	noBuildArgs, err := noBuild.Up()
	if err != nil {
		t.Fatal(err)
	}
	if !hasArg(builtArgs, "--build") || !hasArg(noBuildArgs, "--no-build") {
		t.Fatalf("built=%v no-build=%v", builtArgs, noBuildArgs)
	}
}

func TestBuildWebLifecyclePlanUsesConfiguredDependenciesAndWorkdir(t *testing.T) {
	project := config.Project{
		Services:     map[string]config.Service{"webdashboard": {Language: "typescript", Workdir: "webdashboard"}},
		Dependencies: map[string][]string{"unit-webdashboard": {"bun_init", "webdashboard"}},
	}
	providerConfig := provider.Provider{Name: "fixture", Compose: []string{"fixture-compose"}, Capabilities: []string{"compose", "config", "up", "down", "ps"}}
	plan, service, workdir, err := buildWebLifecyclePlan("/fixture", "isolated-test", providerConfig, project, "webdashboard", "Unit Tests", testplan.Flags{Build: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if service != "webdashboard" || workdir != "/workspace/webdashboard" {
		t.Fatalf("service/workdir = %q/%q", service, workdir)
	}
	args, err := plan.Up()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-p isolated-test", "--profile unit-webdashboard", "webdashboard", "bun_init"} {
		if !strings.Contains(joined, want) {
			t.Errorf("compose args %q missing %q", joined, want)
		}
	}
	down, err := plan.Down()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(down, " "), "-v") {
		t.Fatalf("down removes volumes: %v", down)
	}
}

func TestValidateWebTestFlagsFailsClosed(t *testing.T) {
	for _, flags := range []testplan.Flags{{Pattern: "Test"}, {Coverage: true}, {Fix: true}, {BaseRef: "main"}} {
		if err := validateWebTestFlags(flags); err == nil {
			t.Errorf("accepted unsupported flags: %#v", flags)
		}
	}
	if err := validateWebTestFlags(testplan.Flags{}); err != nil {
		t.Fatal(err)
	}
}

func TestGoServiceUsesSingleServiceTargetAliasForPhaseDependencies(t *testing.T) {
	project := config.Project{
		Services: map[string]config.Service{"apiserver": {Language: "go"}},
		Targets:  map[string][]string{"api": {"apiserver"}, "all": {"apiserver", "webdashboard"}},
		Dependencies: map[string][]string{
			"unit-api": {"apiserver"},
			"int-api":  {"apiserver"},
			"lint-api": {"apiserver"},
		},
	}
	for phase, want := range map[string]string{"Unit Tests": "unit-api", "Integration Tests": "int-api", "Linting": "lint-api"} {
		if got := phaseDependencyKey(project, phase, "apiserver"); got != want {
			t.Errorf("%s dependency key=%q want %q", phase, got, want)
		}
	}
}

func TestGoAliasUsesConfiguredComposeServiceAndProfile(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{"api": {Language: "go", Workdir: "apiserver"}}, Targets: map[string][]string{"api": {"api"}}, Dependencies: map[string][]string{"unit-api": {"apiserver"}, "int-api": {"apiserver"}, "lint-api": {"apiserver"}}}
	for phase, want := range map[string]string{"Unit Tests": "unit-api", "Integration Tests": "int-api", "Linting": "lint-api"} {
		if got := phaseDependencyKey(project, phase, "api"); got != want {
			t.Errorf("%s dependency key=%q want %q", phase, got, want)
		}
	}
}

func TestTestProjectNameIsolatedFromComposeEnvironment(t *testing.T) {
	if got := testProjectName(map[string]string{"COMPOSE_PROJECT_NAME": "consumer"}, "/fixture"); got == "consumer" || !strings.HasPrefix(got, fmt.Sprintf("leedevkit-test-%x-", fnv32a("/fixture"))) {
		t.Fatalf("test project = %q", got)
	}
}
