package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
)

func TestParsePlaywrightMCPArgsAndResolution(t *testing.T) {
	root := t.TempDir()
	vpPath := filepath.Join(root, "config", "ui-viewports.json")
	if err := os.MkdirAll(filepath.Dir(vpPath), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "baseline": "desktop-large",
  "tiers": {
    "required": ["desktop-large", "desktop"],
    "secondary": ["tablet-portrait"],
    "smoke": ["mobile"]
  },
  "supported": [
    {"name": "desktop-large", "width": 1920, "height": 1080},
    {"name": "desktop", "width": 1440, "height": 900},
    {"name": "tablet-portrait", "width": 768, "height": 1024},
    {"name": "mobile", "width": 390, "height": 844}
  ]
}`
	if err := os.WriteFile(vpPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	accountsPath := filepath.Join(root, "docs", "dev-accounts.md")
	if err := os.MkdirAll(filepath.Dir(accountsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accountsPath, []byte("# accounts\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := parsePlaywrightMCPArgs([]string{
		"--endpoint=http://override.local:8080/",
		"--startup-timeout", "20",
		"--", "--trace=on",
	})
	if err != nil {
		t.Fatal(err)
	}

	projectCfg := config.Project{
		PlaywrightMCP: config.PlaywrightMCP{
			Endpoint:           "http://caddy",
			LoginPath:          "/login",
			Network:            "leedevkit-dev_dev_frontend",
			Viewports:          "config/ui-viewports.json",
			AccountsDoc:        "docs/dev-accounts.md",
			StartupTimeout:     10,
			RequestTimeout:     50,
			SessionIdleTimeout: 120,
			ExtraArgs:          []string{"--console-level=warning"},
		},
	}

	rt, err := resolvePlaywrightMCPRuntime(root, projectCfg, opts, map[string]string{
		"PLAYWRIGHT_MCP_NETWORK": "env-net",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rt.Endpoint != "http://override.local:8080" {
		t.Fatalf("endpoint = %q", rt.Endpoint)
	}
	if rt.AllowedHosts != "override.local:8080" || rt.AllowedOrigins != "http://override.local:8080" {
		t.Fatalf("allowed hosts/origins = %q / %q", rt.AllowedHosts, rt.AllowedOrigins)
	}
	if rt.Network != "env-net" {
		t.Fatalf("network = %q", rt.Network)
	}
	if rt.StartupTimeout != 20*time.Second || rt.RequestTimeout != 50*time.Second || rt.SessionIdleTimeout != 120*time.Second {
		t.Fatalf("timeouts = %v / %v / %v", rt.StartupTimeout, rt.RequestTimeout, rt.SessionIdleTimeout)
	}
	if !reflect.DeepEqual(rt.ExtraArgs, []string{"--console-level=warning", "--trace=on"}) {
		t.Fatalf("extra args = %#v", rt.ExtraArgs)
	}
	for _, want := range []string{
		"open http://override.local:8080/login before authentication",
		"To switch roles, call browser_close for a fresh isolated context, then open http://override.local:8080/login",
		"Read docs/dev-accounts.md for test roles and development accounts",
		"Default to desktop-large (1920x1080)",
		"Required desktop: desktop-large=1920x1080, desktop=1440x900",
		"Secondary tablet: tablet-portrait=768x1024",
		"Mobile smoke only: mobile=390x844",
	} {
		if !strings.Contains(rt.Instructions, want) {
			t.Fatalf("instructions missing %q:\n%s", want, rt.Instructions)
		}
	}

	containerArgs := buildPlaywrightContainerArgs([]string{"docker"}, "leedevkit-pw-test", rt)
	joined := strings.Join(containerArgs, " ")
	for _, want := range []string{
		"docker run --rm -i --init --name leedevkit-pw-test --read-only --shm-size=256m",
		"--network env-net",
		"--allowed-hosts=override.local:8080",
		"--allowed-origins=http://override.local:8080",
		"--idle-timeout=0",
		"--console-level=warning --trace=on",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("container args missing %q:\n%s", want, joined)
		}
	}
}

func TestPlaywrightMCPValidationErrors(t *testing.T) {
	if opts, err := parsePlaywrightMCPArgs([]string{"--help"}); err != nil || !opts.Help {
		t.Fatalf("--help = %+v, err=%v", opts, err)
	}
	allFlags, err := parsePlaywrightMCPArgs([]string{
		"--build",
		"--endpoint", "http://app.local",
		"--login-path", "/auth",
		"--network", "net1",
		"--viewports", "vp.json",
		"--accounts-doc", "docs/dev-accounts.md",
		"--image", "img:1",
		"--startup-timeout", "12",
		"--request-timeout", "34",
		"--session-idle-timeout", "56",
	})
	if err != nil || !allFlags.Build || allFlags.LoginPath != "/auth" || allFlags.Network != "net1" ||
		allFlags.Viewports != "vp.json" || allFlags.AccountsDoc != "docs/dev-accounts.md" || allFlags.Image != "img:1" ||
		allFlags.StartupTimeout != 12 || allFlags.RequestTimeout != 34 || allFlags.SessionIdleTimeout != 56 {
		t.Fatalf("allFlags = %+v, err=%v", allFlags, err)
	}
	for _, badArgs := range [][]string{
		{"--unknown"},
		{"--endpoint"},
		{"--login-path"},
		{"--network"},
		{"--viewports"},
		{"--accounts-doc"},
		{"--image"},
		{"--startup-timeout"},
		{"--startup-timeout", "0"},
		{"--request-timeout"},
		{"--request-timeout", "999"},
		{"--session-idle-timeout"},
		{"--session-idle-timeout", "0"},
	} {
		if _, err := parsePlaywrightMCPArgs(badArgs); err == nil {
			t.Fatalf("expected error for %v", badArgs)
		}
	}
	if _, err := resolvePlaywrightMCPRuntime(t.TempDir(), config.Project{}, playwrightCLIOptions{}, map[string]string{}); err == nil {
		t.Fatal("expected error when endpoint is missing")
	}
	if _, err := resolvePlaywrightMCPRuntime(t.TempDir(), config.Project{}, playwrightCLIOptions{Endpoint: "http://user:pass@caddy"}, map[string]string{}); err == nil {
		t.Fatal("expected error when endpoint includes credentials")
	}
	if _, err := resolvePlaywrightMCPRuntime(t.TempDir(), config.Project{}, playwrightCLIOptions{Endpoint: "http://caddy", LoginPath: "noleading"}, map[string]string{}); err == nil {
		t.Fatal("expected error for login_path without leading slash")
	}
	if _, err := resolvePlaywrightMCPRuntime(t.TempDir(), config.Project{}, playwrightCLIOptions{Endpoint: "http://caddy", AccountsDoc: "../escape.md"}, map[string]string{}); err == nil {
		t.Fatal("expected error for accounts_doc path traversal")
	}
	if _, err := resolvePlaywrightMCPRuntime(t.TempDir(), config.Project{}, playwrightCLIOptions{Endpoint: "http://caddy", AccountsDoc: "docs/missing.md"}, map[string]string{}); err == nil {
		t.Fatal("expected error for missing accounts_doc file")
	}
	for _, envKey := range []string{
		"PLAYWRIGHT_MCP_STARTUP_TIMEOUT_SECS",
		"PLAYWRIGHT_MCP_REQUEST_TIMEOUT_SECS",
		"PLAYWRIGHT_MCP_SESSION_IDLE_TIMEOUT_SECS",
	} {
		if _, err := resolvePlaywrightMCPRuntime(t.TempDir(), config.Project{}, playwrightCLIOptions{Endpoint: "http://caddy"}, map[string]string{envKey: "bad"}); err == nil {
			t.Fatalf("expected error for invalid %s", envKey)
		}
	}
	rt, err := resolvePlaywrightMCPRuntime(t.TempDir(), config.Project{}, playwrightCLIOptions{Endpoint: "http://caddy"}, map[string]string{
		"PLAYWRIGHT_MCP_STARTUP_TIMEOUT_SECS":      "1",
		"PLAYWRIGHT_MCP_REQUEST_TIMEOUT_SECS":      "20",
		"PLAYWRIGHT_MCP_SESSION_IDLE_TIMEOUT_SECS": "60",
	})
	if err != nil || rt.StartupTimeout != time.Second || rt.RequestTimeout != 20*time.Second || rt.SessionIdleTimeout != 60*time.Second || rt.Grace != time.Second {
		t.Fatalf("env timeout override = %+v, err=%v", rt, err)
	}

	// Viewport manifest errors.
	if _, err := formatViewportGuidance(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing manifest error")
	}
	for _, badJSON := range []string{
		"{bad",
		`{"baseline":"x","supported":[{"name":"","width":0,"height":0}]}`,
		`{"baseline":"missing","supported":[{"name":"a","width":100,"height":100}]}`,
		`{"baseline":"a","tiers":{"required":["nope"]},"supported":[{"name":"a","width":100,"height":100}]}`,
		`{"baseline":"a","tiers":{"required":["a"],"secondary":["nope"]},"supported":[{"name":"a","width":100,"height":100}]}`,
		`{"baseline":"a","tiers":{"required":["a"],"secondary":["a"],"smoke":["nope"]},"supported":[{"name":"a","width":100,"height":100}]}`,
	} {
		p := filepath.Join(t.TempDir(), "vp.json")
		if err := os.WriteFile(p, []byte(badJSON), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := formatViewportGuidance(p); err == nil {
			t.Fatalf("expected viewport error for %s", badJSON)
		}
	}

	// DEVKIT_HOME asset fallback + missing error.
	home := t.TempDir()
	t.Setenv("DEVKIT_HOME", home)
	if _, err := playwrightAssetDir(t.TempDir()); err == nil {
		t.Fatal("expected missing Dockerfile error")
	}
	dfDir := filepath.Join(home, "current", "container", "playwright-mcp")
	if err := os.MkdirAll(dfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dfDir, "Dockerfile"), []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := playwrightAssetDir(t.TempDir()); err != nil || got != dfDir {
		t.Fatalf("playwrightAssetDir = %q, err=%v", got, err)
	}
}

func TestRunPlaywrightMCPHelpAndCLIDispatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"mcp", "--help"}, strings.NewReader(""), &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "mcp playwright") {
		t.Fatalf("mcp --help: code=%d out=%q err=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"mcp", "playwright", "--help"}, strings.NewReader(""), &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "--build-only") {
		t.Fatalf("mcp playwright --help: code=%d out=%q err=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runPlaywrightMCP([]string{"--bad-flag"}, strings.NewReader(""), &stdout, &stderr, t.TempDir()); code != 2 {
		t.Fatalf("bad flag code = %d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := servePlaywrightStdioProxy(context.Background(), playwrightProxyOptions{Stderr: &stderr}); code != 1 {
		t.Fatalf("empty command code = %d", code)
	}
}

func TestRunPlaywrightMCPMissingImageAndBuildOnly(t *testing.T) {
	root := t.TempDir()
	dockerfileDir := filepath.Join(root, "container", "playwright-mcp")
	if err := os.MkdirAll(dockerfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dockerfileDir, "Dockerfile"), []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	argsLog := filepath.Join(root, "docker-args.log")
	fakeDocker := filepath.Join(root, "fake-docker.sh")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
if [ "$1" = "image" ] && [ "$2" = "inspect" ]; then
  exit 1
fi
exit 0
`, argsLog)
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := fmt.Sprintf(`[container]
provider = "fake"

[container.providers.fake]
cli = [%q]
compose = [%q, "compose"]

[mcp.playwright]
endpoint = "http://caddy"
`, fakeDocker, fakeDocker)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(tomlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runPlaywrightMCP(nil, strings.NewReader(""), &stdout, &stderr, root)
	if code != 1 {
		t.Fatalf("missing image exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Playwright MCP image missing") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must stay empty on error: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runPlaywrightMCP([]string{"--build-only"}, strings.NewReader(""), &stdout, &stderr, root)
	if code != 0 {
		t.Fatalf("--build-only exit code = %d, stderr=%s", code, stderr.String())
	}
	logged, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), "build -t "+defaultPlaywrightMCPImage) {
		t.Fatalf("expected docker build invocation, got:\n%s", string(logged))
	}

	// Now simulate image present + run returning an initialize response.
	scriptOK := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
if [ "$1" = "image" ] && [ "$2" = "inspect" ]; then
  exit 0
fi
if [ "$1" = "run" ]; then
  read -r line
  printf '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}\n'
  exit 0
fi
exit 0
`, argsLog)
	if err := os.WriteFile(fakeDocker, []byte(scriptOK), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runPlaywrightMCP(nil, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n"), &stdout, &stderr, root)
	if code != 0 {
		t.Fatalf("happy-path run exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"instructions":"Use Playwright MCP only for UI tasks.`) {
		t.Fatalf("expected injected instructions in stdout, got: %s", stdout.String())
	}
}

func TestPlaywrightProxyHelperProcess(t *testing.T) {
	mode := os.Getenv("LEEDEVKIT_PW_PROXY_HELPER")
	if mode == "" {
		return
	}
	defer os.Exit(0)

	switch mode {
	case "echo":
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				var req map[string]any
				if json.Unmarshal(line, &req) == nil {
					switch req["method"] {
					case "initialize":
						resp := map[string]any{
							"jsonrpc": "2.0",
							"id":      req["id"],
							"result":  map[string]any{"protocolVersion": "2024-11-05"},
						}
						b, _ := json.Marshal(resp)
						_, _ = os.Stdout.Write(append(b, '\n'))
					case "tools/call":
						big := strings.Repeat("x", 128*1024)
						resp := map[string]any{
							"jsonrpc": "2.0",
							"id":      req["id"],
							"result":  map[string]any{"blob": big},
						}
						b, _ := json.Marshal(resp)
						_, _ = os.Stdout.Write(append(b, '\n'))
					}
				}
			}
			if err != nil {
				return
			}
		}
	case "hang-startup":
		time.Sleep(10 * time.Second)
	case "hang-tool":
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				var req map[string]any
				if json.Unmarshal(line, &req) == nil && req["method"] == "initialize" {
					resp := map[string]any{
						"jsonrpc": "2.0",
						"id":      req["id"],
						"result":  map[string]any{"protocolVersion": "2024-11-05"},
					}
					b, _ := json.Marshal(resp)
					_, _ = os.Stdout.Write(append(b, '\n'))
				}
			}
			if err != nil {
				time.Sleep(10 * time.Second)
				return
			}
		}
	}
}

func helperCommand(t *testing.T, mode string) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEEDEVKIT_PW_PROXY_HELPER", mode)
	return []string{exe, "-test.run=^TestPlaywrightProxyHelperProcess$"}
}

func TestServePlaywrightStdioProxyInjectsInstructionsAndHandlesLargePayload(t *testing.T) {
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"browser_take_screenshot"}}` + "\n",
	)
	var stdout, stderr bytes.Buffer
	code := servePlaywrightStdioProxy(context.Background(), playwrightProxyOptions{
		Command:            helperCommand(t, "echo"),
		Stdin:              input,
		Stdout:             &stdout,
		Stderr:             &stderr,
		Instructions:       "Custom viewport policy",
		StartupTimeout:     5 * time.Second,
		RequestTimeout:     5 * time.Second,
		SessionIdleTimeout: 5 * time.Second,
		Grace:              2 * time.Second,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	reader := bufio.NewReader(&stdout)
	line1, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read init response: %v", err)
	}
	var initResp map[string]any
	if err := json.Unmarshal(line1, &initResp); err != nil {
		t.Fatal(err)
	}
	result := initResp["result"].(map[string]any)
	if result["instructions"] != "Custom viewport policy" {
		t.Fatalf("instructions not injected: %#v", result)
	}
	line2, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read large tool response: %v", err)
	}
	var toolResp map[string]any
	if err := json.Unmarshal(line2, &toolResp); err != nil {
		t.Fatal(err)
	}
	blob := toolResp["result"].(map[string]any)["blob"].(string)
	if len(blob) != 128*1024 {
		t.Fatalf("large response truncated: len=%d", len(blob))
	}
}

func TestServePlaywrightStdioProxyTimeouts(t *testing.T) {
	t.Run("startup timeout", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()
		var stdout, stderr bytes.Buffer
		var cleaned atomic.Int32
		code := servePlaywrightStdioProxy(context.Background(), playwrightProxyOptions{
			Command:            helperCommand(t, "hang-startup"),
			Stdin:              pr,
			Stdout:             &stdout,
			Stderr:             &stderr,
			StartupTimeout:     150 * time.Millisecond,
			RequestTimeout:     5 * time.Second,
			SessionIdleTimeout: 5 * time.Second,
			Grace:              100 * time.Millisecond,
			CleanupContainer:   func() { cleaned.Add(1) },
		})
		if code != playwrightTimeoutExitStatus {
			t.Fatalf("exit code = %d, want 124 (stderr=%s)", code, stderr.String())
		}
		if cleaned.Load() != 1 {
			t.Fatalf("cleanup calls = %d, want 1", cleaned.Load())
		}
	})

	t.Run("request timeout after initialize", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()
		go func() {
			_, _ = pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"))
			time.Sleep(50 * time.Millisecond)
			_, _ = pw.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"slow"}}` + "\n"))
		}()
		var stdout, stderr bytes.Buffer
		var cleaned atomic.Int32
		code := servePlaywrightStdioProxy(context.Background(), playwrightProxyOptions{
			Command:            helperCommand(t, "hang-tool"),
			Stdin:              pr,
			Stdout:             &stdout,
			Stderr:             &stderr,
			StartupTimeout:     2 * time.Second,
			RequestTimeout:     200 * time.Millisecond,
			SessionIdleTimeout: 5 * time.Second,
			Grace:              100 * time.Millisecond,
			CleanupContainer:   func() { cleaned.Add(1) },
		})
		if code != playwrightTimeoutExitStatus {
			t.Fatalf("exit code = %d, want 124 (stderr=%s)", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "request timed out") {
			t.Fatalf("stderr = %s", stderr.String())
		}
		if cleaned.Load() != 1 {
			t.Fatalf("cleanup calls = %d, want 1", cleaned.Load())
		}
	})

	t.Run("session idle timeout after initialize", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()
		go func() {
			_, _ = pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"))
		}()
		var stdout, stderr bytes.Buffer
		var cleaned atomic.Int32
		code := servePlaywrightStdioProxy(context.Background(), playwrightProxyOptions{
			Command:            helperCommand(t, "hang-tool"),
			Stdin:              pr,
			Stdout:             &stdout,
			Stderr:             &stderr,
			StartupTimeout:     2 * time.Second,
			RequestTimeout:     5 * time.Second,
			SessionIdleTimeout: 200 * time.Millisecond,
			Grace:              100 * time.Millisecond,
			CleanupContainer:   func() { cleaned.Add(1) },
		})
		if code != playwrightTimeoutExitStatus {
			t.Fatalf("exit code = %d, want 124 (stderr=%s)", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "session idle") {
			t.Fatalf("stderr = %s", stderr.String())
		}
		if cleaned.Load() != 1 {
			t.Fatalf("cleanup calls = %d, want 1", cleaned.Load())
		}
	})
}
