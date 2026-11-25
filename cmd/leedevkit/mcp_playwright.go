package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/process"
)

const (
	defaultPlaywrightMCPVersion     = "0.0.84"
	defaultPlaywrightMCPImage       = "leedevkit-playwright-mcp:" + defaultPlaywrightMCPVersion
	defaultPlaywrightLoginPath      = "/login"
	defaultPlaywrightStartupTimeout = 15
	defaultPlaywrightRequestTimeout = 45
	defaultPlaywrightSessionIdle    = 300
	playwrightTimeoutExitStatus     = 124
	playwrightBuildTimeout          = 10 * time.Minute
	playwrightInspectTimeout        = 15 * time.Second
	playwrightCleanupTimeout        = 5 * time.Second
)

const playwrightMCPUsage = `Usage: leedevkit mcp playwright [flags] [-- extra-playwright-args...]

Runs a hardened containerized Playwright MCP stdio server with lifecycle watchdog.
Project defaults are read from [mcp.playwright] in leedevkit.toml.

Flags:
  --build                        Build the shared Playwright MCP image before running
  --build-only                   Build the shared Playwright MCP image and exit
  --endpoint URL                 Target application base URL (http/https)
  --login-path PATH              Initial login path appended to endpoint (default: /login)
  --network NAME                 Container network to attach
  --viewports PATH               Path to viewport manifest JSON injected into instructions
  --accounts-doc PATH            Repository-relative doc listing dev roles and accounts
  --image REF                    Override Playwright MCP container image
  --startup-timeout SECS         Max seconds waiting for MCP initialize (default: 15)
  --request-timeout SECS         Max seconds waiting for a pending request (default: 45)
  --session-idle-timeout SECS    Max idle seconds after initialize (default: 300)
`

type playwrightCLIOptions struct {
	Help               bool
	Build              bool
	BuildOnly          bool
	Endpoint           string
	LoginPath          string
	Network            string
	Viewports          string
	AccountsDoc        string
	Image              string
	StartupTimeout     int
	RequestTimeout     int
	SessionIdleTimeout int
	ExtraArgs          []string
}

type playwrightRuntimeConfig struct {
	Build              bool
	BuildOnly          bool
	Endpoint           string
	AllowedHosts       string
	AllowedOrigins     string
	LoginURL           string
	Network            string
	Image              string
	Instructions       string
	StartupTimeout     time.Duration
	RequestTimeout     time.Duration
	SessionIdleTimeout time.Duration
	Grace              time.Duration
	ExtraArgs          []string
}

type viewportManifest struct {
	Baseline string `json:"baseline"`
	Tiers    struct {
		Required  []string `json:"required"`
		Secondary []string `json:"secondary"`
		Smoke     []string `json:"smoke"`
	} `json:"tiers"`
	Supported []struct {
		Name   string `json:"name"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"supported"`
}

func parsePlaywrightMCPArgs(args []string) (playwrightCLIOptions, error) {
	var opts playwrightCLIOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.ExtraArgs = append(opts.ExtraArgs, args[i+1:]...)
			break
		}
		if arg == "--help" || arg == "-h" {
			opts.Help = true
			return opts, nil
		}
		if arg == "--build" {
			opts.Build = true
			continue
		}
		if arg == "--build-only" {
			opts.Build = true
			opts.BuildOnly = true
			continue
		}
		flag, val, hasEq := strings.Cut(arg, "=")
		nextVal := func() (string, error) {
			if hasEq {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", flag)
			}
			i++
			return args[i], nil
		}
		switch flag {
		case "--endpoint":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			opts.Endpoint = v
		case "--login-path":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			opts.LoginPath = v
		case "--network":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			opts.Network = v
		case "--viewports":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			opts.Viewports = v
		case "--accounts-doc":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			opts.AccountsDoc = v
		case "--image":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			opts.Image = v
		case "--startup-timeout":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			parsed, err := parseBoundedInt("--startup-timeout", v, 1, 300)
			if err != nil {
				return opts, err
			}
			opts.StartupTimeout = parsed
		case "--request-timeout":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			parsed, err := parseBoundedInt("--request-timeout", v, 1, 600)
			if err != nil {
				return opts, err
			}
			opts.RequestTimeout = parsed
		case "--session-idle-timeout":
			v, err := nextVal()
			if err != nil {
				return opts, err
			}
			parsed, err := parseBoundedInt("--session-idle-timeout", v, 1, 3600)
			if err != nil {
				return opts, err
			}
			opts.SessionIdleTimeout = parsed
		default:
			return opts, fmt.Errorf("unsupported flag for mcp playwright: %s", arg)
		}
	}
	return opts, nil
}

func parseBoundedInt(name, raw string, minVal, maxVal int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < minVal || n > maxVal {
		return 0, fmt.Errorf("%s must be an integer from %d to %d", name, minVal, maxVal)
	}
	return n, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func resolveBoundedTimeout(defaultSecs, cfgSecs, cliSecs int, envKey string, env map[string]string, maxSecs int) (time.Duration, error) {
	secs := defaultSecs
	if cfgSecs > 0 {
		secs = cfgSecs
	}
	if raw := strings.TrimSpace(env[envKey]); raw != "" {
		n, err := parseBoundedInt(envKey, raw, 1, maxSecs)
		if err != nil {
			return 0, err
		}
		secs = n
	}
	if cliSecs > 0 {
		secs = cliSecs
	}
	return time.Duration(secs) * time.Second, nil
}

func resolvePlaywrightMCPRuntime(root string, projectConfig config.Project, opts playwrightCLIOptions, env map[string]string) (playwrightRuntimeConfig, error) {
	cfg := projectConfig.PlaywrightMCP
	result := playwrightRuntimeConfig{
		Build:     opts.Build,
		BuildOnly: opts.BuildOnly,
		Image:     firstNonEmpty(opts.Image, env["PLAYWRIGHT_MCP_IMAGE"], cfg.Image, defaultPlaywrightMCPImage),
		Network:   firstNonEmpty(opts.Network, env["PLAYWRIGHT_MCP_NETWORK"], cfg.Network),
	}
	if result.BuildOnly {
		return result, nil
	}

	endpoint := firstNonEmpty(opts.Endpoint, env["PLAYWRIGHT_MCP_ENDPOINT"], cfg.Endpoint)
	if endpoint == "" {
		return result, errors.New("Playwright MCP endpoint is required (set [mcp.playwright].endpoint in leedevkit.toml or pass --endpoint)")
	}
	parsed, err := config.ValidatePlaywrightEndpoint(endpoint)
	if err != nil {
		return result, fmt.Errorf("invalid Playwright endpoint %q: %w", endpoint, err)
	}
	result.Endpoint = strings.TrimRight(endpoint, "/")
	result.AllowedHosts = parsed.Host
	result.AllowedOrigins = parsed.Scheme + "://" + parsed.Host

	loginPath := firstNonEmpty(opts.LoginPath, cfg.LoginPath, defaultPlaywrightLoginPath)
	if !strings.HasPrefix(loginPath, "/") {
		return result, fmt.Errorf("invalid Playwright login_path %q: must start with /", loginPath)
	}
	result.LoginURL = result.Endpoint + loginPath

	if result.StartupTimeout, err = resolveBoundedTimeout(defaultPlaywrightStartupTimeout, cfg.StartupTimeout, opts.StartupTimeout, "PLAYWRIGHT_MCP_STARTUP_TIMEOUT_SECS", env, 300); err != nil {
		return result, err
	}
	if result.RequestTimeout, err = resolveBoundedTimeout(defaultPlaywrightRequestTimeout, cfg.RequestTimeout, opts.RequestTimeout, "PLAYWRIGHT_MCP_REQUEST_TIMEOUT_SECS", env, 600); err != nil {
		return result, err
	}
	if result.SessionIdleTimeout, err = resolveBoundedTimeout(defaultPlaywrightSessionIdle, cfg.SessionIdleTimeout, opts.SessionIdleTimeout, "PLAYWRIGHT_MCP_SESSION_IDLE_TIMEOUT_SECS", env, 3600); err != nil {
		return result, err
	}
	result.Grace = 2 * time.Second
	if result.StartupTimeout < result.Grace {
		result.Grace = result.StartupTimeout
	}

	result.ExtraArgs = append(append([]string(nil), cfg.ExtraArgs...), opts.ExtraArgs...)

	instructions := fmt.Sprintf(
		"Use Playwright MCP only for UI tasks. This browser session starts unauthenticated; open %s before authentication. To switch roles, call browser_close for a fresh isolated context, then open %s and sign in as the next role. Never bypass application authorization or inspect secrets, cookies, or storage tokens.",
		result.LoginURL, result.LoginURL,
	)
	accountsDoc := firstNonEmpty(opts.AccountsDoc, env["PLAYWRIGHT_MCP_ACCOUNTS_DOC"], cfg.AccountsDoc)
	if accountsDoc != "" {
		cleanDoc := filepath.Clean(accountsDoc)
		if !config.IsRelativeWithinRoot(accountsDoc) {
			return result, fmt.Errorf("invalid Playwright accounts_doc %q: must be a repository-relative path", accountsDoc)
		}
		if !fileExists(filepath.Join(root, cleanDoc)) {
			return result, fmt.Errorf("Playwright accounts_doc not found: %s", cleanDoc)
		}
		instructions += fmt.Sprintf(" Read %s for test roles and development accounts; select the least-privileged role matching the scenario.", filepath.ToSlash(cleanDoc))
	}
	viewportsPath := firstNonEmpty(opts.Viewports, env["PLAYWRIGHT_MCP_VIEWPORTS"], cfg.Viewports)
	if viewportsPath != "" {
		if !filepath.IsAbs(viewportsPath) {
			viewportsPath = filepath.Join(root, viewportsPath)
		}
		guidance, err := formatViewportGuidance(viewportsPath)
		if err != nil {
			return result, err
		}
		instructions += " " + guidance
	}
	result.Instructions = instructions
	return result, nil
}

func formatViewportGuidance(path string) (string, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("read viewport manifest %s: %w", path, err)
	}
	var manifest viewportManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("parse viewport manifest %s: %w", path, err)
	}
	byName := make(map[string]string, len(manifest.Supported))
	for _, vp := range manifest.Supported {
		if vp.Name == "" || vp.Width <= 0 || vp.Height <= 0 {
			return "", fmt.Errorf("viewport manifest %s has invalid entry %q", path, vp.Name)
		}
		byName[vp.Name] = fmt.Sprintf("%dx%d", vp.Width, vp.Height)
	}
	baselineDims, ok := byName[manifest.Baseline]
	if !ok {
		return "", fmt.Errorf("viewport manifest %s baseline %q not in supported list", path, manifest.Baseline)
	}
	formatTier := func(names []string) (string, error) {
		parts := make([]string, 0, len(names))
		for _, name := range names {
			dims, exists := byName[name]
			if !exists {
				return "", fmt.Errorf("viewport manifest %s references unknown viewport %q", path, name)
			}
			parts = append(parts, fmt.Sprintf("%s=%s", name, dims))
		}
		return strings.Join(parts, ", "), nil
	}
	required, err := formatTier(manifest.Tiers.Required)
	if err != nil {
		return "", err
	}
	secondary, err := formatTier(manifest.Tiers.Secondary)
	if err != nil {
		return "", err
	}
	smoke, err := formatTier(manifest.Tiers.Smoke)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Viewport policy: desktop-first. Default to %s (%s). Required desktop: %s. Secondary tablet: %s. Mobile smoke only: %s. Before each UI scenario, announce selected viewport, call browser_resize, then verify window.innerWidth and window.innerHeight.",
		manifest.Baseline, baselineDims, required, secondary, smoke,
	), nil
}

func playwrightAssetDir(projectRoot string) (string, error) {
	if assetRoot, err := runtimeAssetRoot(); err == nil {
		candidate := filepath.Join(assetRoot, "container", "playwright-mcp")
		if fileExists(filepath.Join(candidate, "Dockerfile")) {
			return candidate, nil
		}
	}
	if projectRoot != "" {
		candidate := filepath.Join(projectRoot, "container", "playwright-mcp")
		if fileExists(filepath.Join(candidate, "Dockerfile")) {
			return candidate, nil
		}
	}
	if home := os.Getenv("DEVKIT_HOME"); home != "" {
		candidate := filepath.Join(home, "current", "container", "playwright-mcp")
		if fileExists(filepath.Join(candidate, "Dockerfile")) {
			return candidate, nil
		}
	}
	return "", errors.New("Playwright MCP Dockerfile not found in global devkit assets (container/playwright-mcp/Dockerfile)")
}

func newPlaywrightContainerName() string {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("leedevkit-pw-%d", time.Now().UnixNano())
	}
	return "leedevkit-pw-" + hex.EncodeToString(raw[:])
}

func buildPlaywrightContainerArgs(cli []string, containerName string, cfg playwrightRuntimeConfig) []string {
	args := append([]string{}, cli...)
	args = append(args,
		"run", "--rm", "-i", "--init",
		"--name", containerName,
		"--read-only",
		"--shm-size=256m",
		"--tmpfs", "/tmp",
		"--tmpfs", "/root/.cache",
		"--tmpfs", "/root/.config",
		"--tmpfs", "/root/.local",
	)
	if cfg.Network != "" {
		args = append(args, "--network", cfg.Network)
	}
	args = append(args,
		cfg.Image,
		"--headless",
		"--isolated",
		"--no-sandbox",
		"--browser=chrome",
		"--caps=vision,pdf,devtools",
		"--console-level=info",
		"--snapshot-boxes",
		"--allowed-hosts="+cfg.AllowedHosts,
		"--allowed-origins="+cfg.AllowedOrigins,
		"--timeout-action=5000",
		"--timeout-navigation=30000",
		"--timeout-settle=1000",
		"--idle-timeout=0",
	)
	args = append(args, cfg.ExtraArgs...)
	return args
}

func runPlaywrightMCP(args []string, stdin io.Reader, stdout, stderr io.Writer, root string) int {
	return runPlaywrightMCPWithRunner(args, stdin, stdout, stderr, root, process.Run)
}

func runPlaywrightMCPWithRunner(args []string, stdin io.Reader, stdout, stderr io.Writer, root string, runner func(context.Context, process.Request) process.Result) int {
	if runner == nil {
		runner = process.Run
	}
	opts, err := parsePlaywrightMCPArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if opts.Help {
		fmt.Fprint(stdout, playwrightMCPUsage)
		return 0
	}

	projectConfig, err := config.Load(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		fmt.Fprintf(stderr, "invalid project config: %v\n", err)
		return 2
	}
	env := environMap(os.Environ())
	rtCfg, err := resolvePlaywrightMCPRuntime(root, projectConfig, opts, env)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	providerConfig, err := resolveProviderForProject(root, env)
	if err != nil {
		fmt.Fprintf(stderr, "invalid container provider: %v\n", err)
		return 2
	}
	cli := providerConfig.CLI

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if rtCfg.Build {
		assetDir, assetErr := playwrightAssetDir(root)
		if assetErr != nil {
			fmt.Fprintln(stderr, assetErr)
			return 1
		}
		buildArgs := append(append([]string{}, cli...), "build", "-t", rtCfg.Image, assetDir)
		res := runner(ctx, process.Request{
			Args:    buildArgs,
			Dir:     root,
			Stdout:  stderr,
			Stderr:  stderr,
			Timeout: playwrightBuildTimeout,
		})
		if res.ExitCode != 0 {
			return res.ExitCode
		}
		if rtCfg.BuildOnly {
			return 0
		}
	} else {
		inspectArgs := append(append([]string{}, cli...), "image", "inspect", rtCfg.Image)
		res := runner(ctx, process.Request{
			Args:    inspectArgs,
			Dir:     root,
			Stdout:  io.Discard,
			Stderr:  io.Discard,
			Timeout: playwrightInspectTimeout,
		})
		if res.ExitCode != 0 {
			fmt.Fprintf(stderr, "Playwright MCP image missing: %s\nBuild it with: leedevkit mcp playwright --build-only\n", rtCfg.Image)
			return 1
		}
	}

	containerName := newPlaywrightContainerName()
	runArgs := buildPlaywrightContainerArgs(cli, containerName, rtCfg)
	cleanup := func() {
		rmArgs := append(append([]string{}, cli...), "rm", "-f", containerName)
		_ = runner(context.Background(), process.Request{
			Args:    rmArgs,
			Dir:     root,
			Stdout:  io.Discard,
			Stderr:  io.Discard,
			Timeout: playwrightCleanupTimeout,
		})
	}
	return servePlaywrightStdioProxy(ctx, playwrightProxyOptions{
		Command:            runArgs,
		Dir:                root,
		Stdin:              stdin,
		Stdout:             stdout,
		Stderr:             stderr,
		Instructions:       rtCfg.Instructions,
		StartupTimeout:     rtCfg.StartupTimeout,
		RequestTimeout:     rtCfg.RequestTimeout,
		SessionIdleTimeout: rtCfg.SessionIdleTimeout,
		Grace:              rtCfg.Grace,
		CleanupContainer:   cleanup,
	})
}
