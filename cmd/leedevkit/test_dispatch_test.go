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
	execCount := 0
	runner := func(_ context.Context, request process.Request) process.Result {
		commands = append(commands, request.Args)
		if hasArg(request.Args, "exec") {
			execCount++
			if execCount == 3 {
				return process.Result{ExitCode: 7}
			}
		}
		return process.Result{}
	}
	result := runInfra(t.TempDir(), testplan.Flags{Target: "infra"}, runner, nil)
	if result.Status != "failed" || result.ExitCode != 7 || result.FullRegression {
		t.Fatalf("result = %#v", result)
	}
	if execCount != 12 {
		t.Fatalf("ran %d exec checks, want 12", execCount)
	}
	if len(commands) != 14 {
		t.Fatalf("ran %d compose commands (up + 12 exec + down), want 14", len(commands))
	}
	if !hasArg(commands[0], "up") || hasArg(commands[0], "--build") {
		t.Fatalf("first command = %v, want up without --build", commands[0])
	}
	if !hasArg(commands[len(commands)-1], "down") {
		t.Fatalf("last command = %v, want down", commands[len(commands)-1])
	}
}

func TestInfraRunsGoQualityGatesThroughCompose(t *testing.T) {
	var commands []string
	runInfra(t.TempDir(), testplan.Flags{Target: "infra"}, func(_ context.Context, request process.Request) process.Result {
		commands = append(commands, strings.Join(request.Args, " "))
		return process.Result{}
	}, nil)
	joined := strings.Join(commands, "\n")
	for _, check := range []string{"go test ./...", "gofmt -l", "go vet ./...", "go test -coverpkg=./..."} {
		if !strings.Contains(joined, check) {
			t.Errorf("missing %s gate", check)
		}
	}
	if len(commands) != 14 {
		t.Fatalf("commands = %d, want 14 (up + 12 exec + down)", len(commands))
	}
	if !strings.Contains(commands[0], "--profile infra up -d --wait --wait-timeout 120 infra") || strings.Contains(commands[0], "--build") {
		t.Errorf("invalid Compose up invocation: %s", commands[0])
	}
	for _, command := range commands[1 : len(commands)-1] {
		if strings.HasPrefix(command, "docker run ") {
			t.Errorf("Go quality check bypasses Compose: %s", command)
		}
		if !strings.Contains(command, "exec -T -w /workspace infra") || !strings.Contains(command, "-f ") {
			t.Errorf("invalid Compose Go check invocation: %s", command)
		}
	}
	if !strings.Contains(commands[len(commands)-1], "down --remove-orphans --timeout 1") {
		t.Errorf("invalid Compose down invocation: %s", commands[len(commands)-1])
	}
}

func TestInfraBuildFlagAddsBuildOnlyToUp(t *testing.T) {
	var commands []string
	runInfra(t.TempDir(), testplan.Flags{Target: "infra", Build: true}, func(_ context.Context, request process.Request) process.Result {
		commands = append(commands, strings.Join(request.Args, " "))
		return process.Result{}
	}, nil)
	if len(commands) != 14 {
		t.Fatalf("commands = %d, want 14", len(commands))
	}
	if !strings.Contains(commands[0], "--profile infra up -d --build --wait --wait-timeout 120 infra") {
		t.Fatalf("up command missing --build: %s", commands[0])
	}
	for _, command := range commands[1:] {
		if strings.Contains(command, "--build") {
			t.Fatalf("subsequent command must not rebuild: %s", command)
		}
	}
}

func TestInfraComposeUsesProjectAndRoot(t *testing.T) {
	root := t.TempDir()
	got := infraComposeCommand(root, "leedevkit-infra-test", filepath.Join(root, "container", "go", "docker-compose.test.yml"), []string{"go", "test", "./..."})
	joined := strings.Join(got, " ")
	for _, want := range []string{"compose", "-p", "leedevkit-infra-test", filepath.Join(root, "container", "go", "docker-compose.test.yml"), "exec -T -w /workspace infra go test ./..."} {
		if !strings.Contains(joined, want) {
			t.Errorf("Compose command %q does not contain %q", joined, want)
		}
	}
}

func TestInfraAllChecksUseComposeService(t *testing.T) {
	root := t.TempDir()
	commands := runInfraCommands(root, "leedevkit-infra-test", filepath.Join(root, "container", "go", "docker-compose.test.yml"))
	if len(commands) != 12 {
		t.Fatalf("checks = %d, want 12", len(commands))
	}
	for _, command := range commands {
		joined := strings.Join(command, " ")
		if !strings.HasPrefix(joined, "docker compose ") || !strings.Contains(joined, "exec -T -w /workspace infra") {
			t.Errorf("check bypasses Compose infra service: %v", command)
		}
	}
}

func TestInfraRejectsUnrecognizedFlags(t *testing.T) {
	for _, flag := range []string{"--typo", "--dry-run", "--unit-only", "--skip-build"} {
		if err := validateInfraFlags([]string{flag}); err == nil {
			t.Errorf("accepted %s", flag)
		}
	}
	for _, flag := range []string{"--json", "--quiet", "--json-stream", "--build"} {
		if err := validateInfraFlags([]string{flag}); err != nil {
			t.Errorf("rejected %s: %v", flag, err)
		}
	}
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

func TestGoTestBuildFlagControlsComposeBuildOnce(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "compose-fixture")
	logPath := filepath.Join(root, "compose.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$COMPOSE_LOG\"\nfor arg do\n case \"$arg\" in ps) printf '[{\"Service\":\"backend\",\"State\":\"running\"}]\\n'; exit;; esac\ndone\n"
	if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := "[container]\nprovider = 'fixture'\n[container.providers.fixture]\ncli = ['fixture-engine']\ncompose = ['" + engine + "']\n[services.backend]\nlang = 'go'\n[targets]\nall = ['go']\n[quality.gates.u]\nphase = 'unit'\ncommand = ['ok']\nrequired = true\n[quality.gates.l]\nphase = 'lint'\ncommand = ['ok']\nrequired = true\n[quality.go]\nunit = ['u']\nlint = ['l']\n[test]\nbuild_policy = 'once'\n"
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".compose"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("COMPOSE_LOG", logPath)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"test", "all", "--skip-e2e", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("default run exit=%d stderr=%q", code, stderr.String())
	}
	defaultLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(defaultLog), "--build") || strings.Contains(string(defaultLog), "--no-build") {
		t.Fatalf("default run must omit build flags: %s", defaultLog)
	}

	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"test", "all", "--skip-e2e", "--build", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("--build run exit=%d stderr=%q", code, stderr.String())
	}
	buildLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(buildLog), "up -d --build"); count != 1 {
		t.Fatalf("build_policy=once with --build ran `up -d --build` %d times, want 1:\n%s", count, buildLog)
	}

	failScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$COMPOSE_LOG\"\nfor arg do\n case \"$arg\" in ps) printf '[{\"Service\":\"backend\",\"State\":\"running\"}]\\n'; exit;; fail-unit) exit 1;; esac\ndone\n"
	if err := os.WriteFile(engine, []byte(failScript), 0o700); err != nil {
		t.Fatal(err)
	}
	failCfg := strings.Replace(cfg, "command = ['ok']\nrequired = true\n[quality.gates.l]", "command = ['fail-unit']\nrequired = true\n[quality.gates.l]", 1)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(failCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"test", "all", "--skip-e2e", "--build", "--json"}, strings.NewReader(""), &stdout, &stderr); code == 0 {
		t.Fatalf("failing phase 1 unexpectedly succeeded: stdout=%q", stdout.String())
	}
	failLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(failLog), "up -d --build"); count != 1 {
		t.Fatalf("build_policy=once with failing phase 1 ran `up -d --build` %d times, want 1:\n%s", count, failLog)
	}
}
