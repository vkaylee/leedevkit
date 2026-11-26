package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestParseExecArgsValidation(t *testing.T) {
	opts, err := parseExecArgs([]string{"exec", "--dev", "-w", "webdashboard", "web", "--", "bun", "run", "gen-api"})
	if err != nil || !opts.Dev || opts.Workdir != "webdashboard" || opts.Service != "web" || len(opts.Command) != 3 || opts.Command[2] != "gen-api" {
		t.Fatalf("opts=%#v err=%v", opts, err)
	}
	helpOpts, err := parseExecArgs([]string{"exec", "--help"})
	if err != nil || !helpOpts.Help {
		t.Fatalf("help opts=%#v err=%v", helpOpts, err)
	}
	for _, bad := range [][]string{
		{"exec"},
		{"exec", "webdashboard"},
		{"exec", "webdashboard", "--"},
		{"exec", "--unknown", "webdashboard", "ls"},
		{"exec", "-w"},
	} {
		if _, err := parseExecArgs(bad); err == nil {
			t.Errorf("expected error for %v", bad)
		}
	}
}

func TestHermeticExecStartsManagedContainerAndSetsWorkdir(t *testing.T) {
	root := t.TempDir()
	compose := filepath.Join(root, "fake-compose")
	manifest := fmt.Sprintf(`
[project]
namespace = "acme"
[container]
provider = "fixture"
[container.providers.fixture]
cli = [%q]
compose = [%q]
capabilities = ["compose", "config", "up", "down", "ps", "exec"]
[services.apiserver]
lang = "go"
workdir = "apiserver"
[services.webdashboard]
lang = "typescript"
dockerfile = "./webdashboard"
[targets]
api = ["apiserver"]
web = ["webdashboard"]
[test.dependencies]
lint-api = ["apiserver"]
lint-web = ["bun_init", "webdashboard"]
`, compose, compose)
	if err := os.WriteFile(compose, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".compose"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")

	t.Run("ephemeral start -> readiness -> exec in workdir -> clean stdout -> teardown", func(t *testing.T) {
		var calls []string
		var gotTestProject string
		psCount := 0
		var stdout, stderr bytes.Buffer
		code := runExecContext(
			context.Background(),
			[]string{"exec", "web", "--", "bun", "run", "gen-api"},
			root,
			[]string{"LEEDEVKIT_CONTAINER_PROVIDER=fixture"},
			strings.NewReader(""),
			&stdout,
			&stderr,
			func(_ context.Context, req process.Request) process.Result {
				joined := strings.Join(req.Args, " ")
				calls = append(calls, joined)
				envMap := environMap(req.Env)
				gotTestProject = envMap["LEEDEVKIT_TEST_PROJECT"]
				switch {
				case strings.Contains(joined, " ps --format json"):
					psCount++
					if psCount == 1 {
						// Initially not running
						return process.Result{ExitCode: 0, Stdout: []byte("[]\n")}
					}
					return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"webdashboard","State":"running","Health":"healthy"}]`)}
				case strings.Contains(joined, " up -d"):
					if req.Stdout != nil {
						_, _ = req.Stdout.Write([]byte("compose up noise\n"))
					}
					return process.Result{ExitCode: 0}
				case strings.Contains(joined, " exec -T -w /workspace/webdashboard webdashboard bun run gen-api"):
					if req.Stdout != nil {
						_, _ = req.Stdout.Write([]byte("generated-types-ok\n"))
					}
					return process.Result{ExitCode: 0}
				default:
					return process.Result{ExitCode: 0}
				}
			},
		)
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr.String())
		}
		if stdout.String() != "generated-types-ok\n" {
			t.Fatalf("stdout polluted by lifecycle logs: %q", stdout.String())
		}
		if gotTestProject != "acme-test" {
			t.Fatalf("LEEDEVKIT_TEST_PROJECT = %q, want acme-test", gotTestProject)
		}
		joinedCalls := strings.Join(calls, "\n")
		for _, want := range []string{
			"-p acme-test",
			"--profile lint-web up -d webdashboard bun_init",
			"exec -T -w /workspace/webdashboard webdashboard bun run gen-api",
			"down --remove-orphans --timeout 1",
		} {
			if !strings.Contains(joinedCalls, want) {
				t.Errorf("calls missing %q:\n%s", want, joinedCalls)
			}
		}
	})

	t.Run("reuses already running container without up or down", func(t *testing.T) {
		var calls []string
		var stdout, stderr bytes.Buffer
		code := runExecContext(
			context.Background(),
			[]string{"exec", "apiserver", "--", "go", "generate", "./..."},
			root,
			[]string{"LEEDEVKIT_CONTAINER_PROVIDER=fixture"},
			strings.NewReader(""),
			&stdout,
			&stderr,
			func(_ context.Context, req process.Request) process.Result {
				joined := strings.Join(req.Args, " ")
				calls = append(calls, joined)
				if strings.Contains(joined, " ps --format json") {
					return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"apiserver","State":"running","Health":"healthy"}]`)}
				}
				return process.Result{ExitCode: 5}
			},
		)
		if code != 5 {
			t.Fatalf("expected propagated exit code 5, got %d", code)
		}
		joinedCalls := strings.Join(calls, "\n")
		if strings.Contains(joinedCalls, " up ") || strings.Contains(joinedCalls, " down ") {
			t.Fatalf("should reuse running container without up/down:\n%s", joinedCalls)
		}
		if !strings.Contains(joinedCalls, "exec -T -w /workspace/apiserver apiserver go generate ./...") {
			t.Fatalf("unexpected exec argv:\n%s", joinedCalls)
		}
	})

	t.Run("dev mode starts dev stack when stopped and rejects unknown service", func(t *testing.T) {
		var calls []string
		psCalls := 0
		var stdout, stderr bytes.Buffer
		code := runExecContext(
			context.Background(),
			[]string{"exec", "--dev", "api", "--", "go", "version"},
			root,
			[]string{"LEEDEVKIT_CONTAINER_PROVIDER=fixture"},
			strings.NewReader(""),
			&stdout,
			&stderr,
			func(_ context.Context, req process.Request) process.Result {
				joined := strings.Join(req.Args, " ")
				calls = append(calls, joined)
				if strings.Contains(joined, " ps --format json") {
					psCalls++
					if psCalls == 1 {
						return process.Result{ExitCode: 0, Stdout: []byte("[]")}
					}
					return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"apiserver","State":"running"}]`)}
				}
				return process.Result{ExitCode: 0}
			},
		)
		if code != 0 || !strings.Contains(strings.Join(calls, "\n"), "-p acme-dev") {
			t.Fatalf("code=%d calls=%v stderr=%q", code, calls, stderr.String())
		}

		stderr.Reset()
		badCode := runExecContext(context.Background(), []string{"exec", "nonexistent", "--", "ls"}, root, []string{"LEEDEVKIT_CONTAINER_PROVIDER=fixture"}, strings.NewReader(""), &stdout, &stderr, nil)
		if badCode != 2 || !strings.Contains(stderr.String(), "not configured") {
			t.Fatalf("expected unknown service error, got code=%d stderr=%q", badCode, stderr.String())
		}
	})
}

func TestEnsureBuiltInGoCacheVolumes(t *testing.T) {
	var created []string
	runner := func(_ context.Context, req process.Request) process.Result {
		created = append(created, strings.Join(req.Args, " "))
		return process.Result{}
	}
	ensureBuiltInGoCacheVolumes(context.Background(), []string{"docker"}, []string{"/repo/.compose/docker-compose.test.yml"}, "/repo", nil, runner)
	if len(created) != 0 {
		t.Fatalf("custom compose file should not create built-in volumes: %v", created)
	}
	ensureBuiltInGoCacheVolumes(context.Background(), []string{"docker"}, []string{"/repo/container/go/docker-compose.test.yml"}, "/repo", nil, runner)
	want := strings.Join([]string{
		"docker volume create leedevkit_go-module-cache",
		"docker volume create leedevkit_go-build-cache",
	}, "\n")
	if strings.Join(created, "\n") != want {
		t.Fatalf("created = %v, want %q", created, want)
	}
}
