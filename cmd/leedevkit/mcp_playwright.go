package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return result, fmt.Errorf("invalid Playwright endpoint %q: must be a valid http(s) URL", endpoint)
	}
	if parsed.User != nil {
		return result, errors.New("Playwright endpoint must not include credentials")
	}
	result.Endpoint = strings.TrimRight(endpoint, "/")
	result.AllowedHosts = parsed.Host
	result.AllowedOrigins = parsed.Scheme + "://" + parsed.Host

	loginPath := firstNonEmpty(opts.LoginPath, cfg.LoginPath, defaultPlaywrightLoginPath)
	if !strings.HasPrefix(loginPath, "/") {
		return result, fmt.Errorf("invalid Playwright login_path %q: must start with /", loginPath)
	}
	result.LoginURL = result.Endpoint + loginPath

	startupSecs := defaultPlaywrightStartupTimeout
	if cfg.StartupTimeout > 0 {
		startupSecs = cfg.StartupTimeout
	}
	if raw := strings.TrimSpace(env["PLAYWRIGHT_MCP_STARTUP_TIMEOUT_SECS"]); raw != "" {
		n, err := parseBoundedInt("PLAYWRIGHT_MCP_STARTUP_TIMEOUT_SECS", raw, 1, 300)
		if err != nil {
			return result, err
		}
		startupSecs = n
	}
	if opts.StartupTimeout > 0 {
		startupSecs = opts.StartupTimeout
	}

	requestSecs := defaultPlaywrightRequestTimeout
	if cfg.RequestTimeout > 0 {
		requestSecs = cfg.RequestTimeout
	}
	if raw := strings.TrimSpace(env["PLAYWRIGHT_MCP_REQUEST_TIMEOUT_SECS"]); raw != "" {
		n, err := parseBoundedInt("PLAYWRIGHT_MCP_REQUEST_TIMEOUT_SECS", raw, 1, 600)
		if err != nil {
			return result, err
		}
		requestSecs = n
	}
	if opts.RequestTimeout > 0 {
		requestSecs = opts.RequestTimeout
	}

	idleSecs := defaultPlaywrightSessionIdle
	if cfg.SessionIdleTimeout > 0 {
		idleSecs = cfg.SessionIdleTimeout
	}
	if raw := strings.TrimSpace(env["PLAYWRIGHT_MCP_SESSION_IDLE_TIMEOUT_SECS"]); raw != "" {
		n, err := parseBoundedInt("PLAYWRIGHT_MCP_SESSION_IDLE_TIMEOUT_SECS", raw, 1, 3600)
		if err != nil {
			return result, err
		}
		idleSecs = n
	}
	if opts.SessionIdleTimeout > 0 {
		idleSecs = opts.SessionIdleTimeout
	}

	result.StartupTimeout = time.Duration(startupSecs) * time.Second
	result.RequestTimeout = time.Duration(requestSecs) * time.Second
	result.SessionIdleTimeout = time.Duration(idleSecs) * time.Second
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
		if filepath.IsAbs(cleanDoc) || cleanDoc == ".." || strings.HasPrefix(cleanDoc, ".."+string(filepath.Separator)) {
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
		res := process.Run(ctx, process.Request{
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
		res := process.Run(ctx, process.Request{
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
		_ = process.Run(context.Background(), process.Request{
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

type playwrightProxyOptions struct {
	Command            []string
	Dir                string
	Stdin              io.Reader
	Stdout             io.Writer
	Stderr             io.Writer
	Instructions       string
	StartupTimeout     time.Duration
	RequestTimeout     time.Duration
	SessionIdleTimeout time.Duration
	Grace              time.Duration
	CleanupContainer   func()
}

type stdioLine struct {
	data []byte
	eof  bool
	err  error
}

func readLines(r io.Reader, ch chan<- stdioLine) {
	defer close(ch)
	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			ch <- stdioLine{data: line}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				ch <- stdioLine{eof: true}
			} else {
				ch <- stdioLine{err: err}
			}
			return
		}
	}
}

func parseJSONRPCMessages(line []byte) ([]map[string]any, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, false
	}
	var single map[string]any
	if err := json.Unmarshal(trimmed, &single); err == nil && single != nil {
		return []map[string]any{single}, false
	}
	var batch []map[string]any
	if err := json.Unmarshal(trimmed, &batch); err == nil && len(batch) > 0 {
		return batch, true
	}
	return nil, false
}

func canonicalMessageID(msg map[string]any) (string, bool) {
	raw, ok := msg["id"]
	if !ok {
		return "", false
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func stopTimer(t *time.Timer) {
	if t == nil {
		return
	}
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	if l.w == nil {
		return len(p), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func servePlaywrightStdioProxy(ctx context.Context, opts playwrightProxyOptions) int {
	safeStderr := &lockedWriter{w: opts.Stderr}
	if len(opts.Command) == 0 {
		fmt.Fprintln(safeStderr, "Playwright MCP command is empty")
		return 1
	}
	grace := opts.Grace
	if grace <= 0 {
		grace = 2 * time.Second
	}

	childStdoutR, childStdoutW, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(safeStderr, "Playwright MCP pipe error: %v\n", err)
		return 1
	}
	cmd := exec.Command(opts.Command[0], opts.Command[1:]...)
	cmd.Dir = opts.Dir
	cmd.Stdout = childStdoutW
	cmd.Stderr = safeStderr
	process.ConfigureProcessGroup(cmd)

	childStdin, err := cmd.StdinPipe()
	if err != nil {
		_ = childStdoutR.Close()
		_ = childStdoutW.Close()
		fmt.Fprintf(safeStderr, "Playwright MCP stdin pipe error: %v\n", err)
		return 1
	}
	if err := cmd.Start(); err != nil {
		_ = childStdin.Close()
		_ = childStdoutR.Close()
		_ = childStdoutW.Close()
		fmt.Fprintf(safeStderr, "start Playwright MCP: %v\n", err)
		return 127
	}
	_ = childStdoutW.Close()
	defer childStdoutR.Close()

	reqCh := make(chan stdioLine, 16)
	respCh := make(chan stdioLine, 16)
	waitCh := make(chan error, 1)
	go readLines(opts.Stdin, reqCh)
	go readLines(childStdoutR, respCh)
	go func() { waitCh <- cmd.Wait() }()

	var (
		initialized   bool
		terminating   bool
		timedOut      bool
		stdinClosed   bool
		pending       = map[string]struct{}{}
		initializeIDs = map[string]struct{}{}
		toolIDs       = map[string]struct{}{}
		startupTimer  *time.Timer
		requestTimer  *time.Timer
		idleTimer     *time.Timer
		eofTimer      *time.Timer
		killTimer     *time.Timer
		startupCh     <-chan time.Time
		requestCh     <-chan time.Time
		idleCh        <-chan time.Time
		eofCh         <-chan time.Time
		killCh        <-chan time.Time
		waitErr       error
		exited        bool
	)

	closeChildStdin := func() {
		if !stdinClosed {
			stdinClosed = true
			_ = childStdin.Close()
		}
	}

	armStartup := func() {
		if opts.StartupTimeout > 0 {
			startupTimer = time.NewTimer(opts.StartupTimeout)
			startupCh = startupTimer.C
		}
	}
	armRequest := func() {
		if opts.RequestTimeout <= 0 {
			return
		}
		if requestTimer == nil {
			requestTimer = time.NewTimer(opts.RequestTimeout)
		} else {
			stopTimer(requestTimer)
			requestTimer.Reset(opts.RequestTimeout)
		}
		requestCh = requestTimer.C
	}
	disarmRequest := func() {
		stopTimer(requestTimer)
		requestCh = nil
	}
	armIdle := func() {
		if opts.SessionIdleTimeout <= 0 {
			return
		}
		if idleTimer == nil {
			idleTimer = time.NewTimer(opts.SessionIdleTimeout)
		} else {
			stopTimer(idleTimer)
			idleTimer.Reset(opts.SessionIdleTimeout)
		}
		idleCh = idleTimer.C
	}
	terminate := func() {
		if terminating {
			return
		}
		terminating = true
		closeChildStdin()
		stopTimer(startupTimer)
		startupCh = nil
		disarmRequest()
		stopTimer(idleTimer)
		idleCh = nil
		stopTimer(eofTimer)
		eofCh = nil
		_ = process.SignalProcessGroup(cmd.Process, false)
		killTimer = time.NewTimer(grace)
		killCh = killTimer.C
	}

	defer func() {
		stopTimer(startupTimer)
		stopTimer(requestTimer)
		stopTimer(idleTimer)
		stopTimer(eofTimer)
		stopTimer(killTimer)
		closeChildStdin()
	}()

	armStartup()

	handleResponseLine := func(raw []byte) {
		msgs, isBatch := parseJSONRPCMessages(raw)
		modified := false
		for _, msg := range msgs {
			id, hasID := canonicalMessageID(msg)
			if !hasID {
				continue
			}
			if _, hasMethod := msg["method"]; hasMethod {
				continue
			}
			if _, ok := pending[id]; !ok {
				continue
			}
			delete(pending, id)
			if _, isTool := toolIDs[id]; isTool {
				delete(toolIDs, id)
				if initialized {
					armIdle()
				}
			}
			if _, isInit := initializeIDs[id]; !isInit {
				continue
			}
			delete(initializeIDs, id)
			resMap, ok := msg["result"].(map[string]any)
			if !ok {
				continue
			}
			initialized = true
			stopTimer(startupTimer)
			startupCh = nil
			armIdle()
			if opts.Instructions != "" {
				resMap["instructions"] = opts.Instructions
				modified = true
			}
		}

		outgoing := raw
		if modified {
			var payload any = msgs[0]
			if isBatch {
				payload = msgs
			}
			if encoded, encErr := json.Marshal(payload); encErr == nil {
				outgoing = append(encoded, '\n')
			}
		}
		_, _ = opts.Stdout.Write(outgoing)

		if initialized {
			if len(pending) == 0 {
				disarmRequest()
			} else {
				armRequest()
			}
		}
	}

	cleanupRan := false
	runCleanup := func() {
		if cleanupRan || opts.CleanupContainer == nil {
			return
		}
		cleanupRan = true
		opts.CleanupContainer()
	}

	for !exited {
		select {
		case waitErr = <-waitCh:
			exited = true
		case <-ctx.Done():
			terminate()
		case ev, ok := <-reqCh:
			if !ok || ev.eof || ev.err != nil {
				reqCh = nil
				closeChildStdin()
				if !terminating && eofTimer == nil {
					eofTimer = time.NewTimer(grace)
					eofCh = eofTimer.C
				}
				continue
			}
			if terminating {
				continue
			}
			if _, wErr := childStdin.Write(ev.data); wErr != nil {
				terminate()
				continue
			}
			msgs, _ := parseJSONRPCMessages(ev.data)
			for _, msg := range msgs {
				id, hasID := canonicalMessageID(msg)
				method, hasMethod := msg["method"].(string)
				if !hasID || !hasMethod {
					continue
				}
				if initialized && len(pending) == 0 {
					armRequest()
				}
				switch method {
				case "initialize":
					initializeIDs[id] = struct{}{}
				case "tools/call":
					toolIDs[id] = struct{}{}
				}
				pending[id] = struct{}{}
			}
		case ev, ok := <-respCh:
			if !ok || ev.eof || ev.err != nil {
				respCh = nil
				if !terminating {
					terminate()
				}
				continue
			}
			handleResponseLine(ev.data)
		case <-startupCh:
			startupCh = nil
			if !initialized {
				timedOut = true
				fmt.Fprintf(safeStderr, "Playwright MCP startup timed out after %s; sent TERM with %s cleanup grace.\n", opts.StartupTimeout, grace)
				terminate()
			}
		case <-requestCh:
			requestCh = nil
			if len(pending) > 0 {
				timedOut = true
				fmt.Fprintf(safeStderr, "Playwright MCP request timed out after %s; sent TERM with %s cleanup grace.\n", opts.RequestTimeout, grace)
				terminate()
			}
		case <-idleCh:
			idleCh = nil
			if initialized && len(pending) == 0 {
				timedOut = true
				fmt.Fprintf(safeStderr, "Playwright MCP session idle for %s; sent TERM with %s cleanup grace.\n", opts.SessionIdleTimeout, grace)
				terminate()
			}
		case <-eofCh:
			eofCh = nil
			terminate()
		case <-killCh:
			killCh = nil
			_ = process.SignalProcessGroup(cmd.Process, true)
			runCleanup()
			waitErr = <-waitCh
			exited = true
		}
	}

	// Drain any buffered response lines flushed before the child exited.
	if respCh != nil {
		for ev := range respCh {
			if len(ev.data) > 0 {
				handleResponseLine(ev.data)
			}
		}
	}
	if terminating || timedOut {
		_ = process.SignalProcessGroup(cmd.Process, true)
		runCleanup()
	}

	if timedOut {
		return playwrightTimeoutExitStatus
	}
	if waitErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}
