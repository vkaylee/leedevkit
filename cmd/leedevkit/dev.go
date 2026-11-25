package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

const devUsage = `Usage: leedevkit dev <subcommand> [args...]

Subcommands:
  up                         Start dev environment
  down                       Stop dev environment
  clean                      Stop dev environment and remove volumes
  ps                         List dev services
  logs [service]             Follow dev service logs
  exec <service> [--] [cmd]  Run command in running dev service
  restart <service>          Restart service and wait for /health readiness
  watch [service]            Watch Go source changes and auto-restart service
  migrate [up|status]        Run incremental migrations or check drift on dev DB
`

var safeMigrationName = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.sql$`)

func devProjectName(env map[string]string, projectConfig config.Project) string {
	if value := strings.TrimSpace(env["LEEDEVKIT_DEV_PROJECT"]); value != "" {
		return value
	}
	if value := strings.TrimSpace(env["COMPOSE_PROJECT_NAME"]); value != "" {
		return value
	}
	if projectConfig.Namespace != "" {
		return projectConfig.Namespace + "-dev"
	}
	return "leedevkit-dev"
}

func devComposeFiles(root string) []string {
	if root != "" && fileExists(filepath.Join(root, "docker-compose.yml")) && !fileExists(filepath.Join(root, ".compose", "docker-compose.dev.yml")) {
		return []string{"docker-compose.yml"}
	}
	return []string{"docker-compose.yml", ".compose/docker-compose.dev.yml"}
}

func devComposeArgs(compose []string, operation string) ([]string, error) {
	return devComposeArgsWithOptions(compose, operation, "", nil)
}

func devComposeArgsWithOptions(compose []string, operation, service string, command []string) ([]string, error) {
	return devComposeArgsForProject(compose, "leedevkit-dev", []string{"docker-compose.yml", ".compose/docker-compose.dev.yml"}, operation, service, command)
}

func devComposeArgsForProject(compose []string, project string, files []string, operation, service string, command []string) ([]string, error) {
	if operation != "up" && operation != "down" && operation != "ps" && operation != "logs" && operation != "exec" && operation != "clean" && operation != "restart" {
		return nil, fmt.Errorf("unsupported dev operation %q", operation)
	}
	if project == "" {
		project = "leedevkit-dev"
	}
	if len(files) == 0 {
		files = []string{"docker-compose.yml", ".compose/docker-compose.dev.yml"}
	}
	args := append([]string{}, compose...)
	composeOperation := operation
	if operation == "clean" {
		composeOperation = "down"
	}
	args = append(args, "-p", project)
	for _, file := range files {
		args = append(args, "-f", file)
	}
	args = append(args, composeOperation)
	switch operation {
	case "up":
		args = append(args, "-d")
	case "clean":
		args = append(args, "-v")
	case "logs":
		args = append(args, "-f")
		if service != "" {
			args = append(args, service)
		}
	case "restart":
		if service == "" {
			return nil, fmt.Errorf("dev restart requires service")
		}
		args = append(args, service)
	case "exec":
		if service == "" {
			return nil, fmt.Errorf("dev exec requires service")
		}
		args = append(args, service)
		if len(command) > 0 && command[0] == "--" {
			command = command[1:]
		}
		if len(command) == 0 {
			args = append(args, "bash")
		} else {
			args = append(args, command...)
		}
	}
	return args, nil
}

func runDev(operation string, providerCfg provider.Provider, root string, stdout, stderr io.Writer, run func(context.Context, process.Request) process.Result) int {
	projectCfg, _ := config.Load(filepath.Join(root, "leedevkit.toml"))
	project := devProjectName(environMap(os.Environ()), projectCfg)
	args, err := devComposeArgsForProject(providerCfg.Compose, project, devComposeFiles(root), operation, "", nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if run == nil {
		run = process.Run
	}
	env := withDevProjectEnv(os.Environ(), root, project, projectCfg.Namespace)
	result := run(context.Background(), process.Request{Args: args, Dir: root, Env: env, Stdout: stdout, Stderr: stderr})
	if result.ExitCode != 0 && result.Err != nil {
		fmt.Fprintln(stderr, result.Err)
	}
	if result.ExitCode == 0 && operation == "up" && projectCfg.AutoMigrate {
		return runDevMigrateContext(context.Background(), nil, providerCfg, projectCfg, root, os.Environ(), stdout, stderr, run)
	}
	return result.ExitCode
}

func resolveDevService(projectCfg config.Project, requested string) (string, config.Service) {
	if svc, ok := projectCfg.Services[requested]; ok {
		return requested, svc
	}
	if targets := projectCfg.Targets[requested]; len(targets) == 1 {
		if svc, ok := projectCfg.Services[targets[0]]; ok {
			return targets[0], svc
		}
	}
	return requested, config.Service{}
}

func defaultHTTPHealthCheck(ctx context.Context, url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return nil
}

func waitHTTPHealthy(ctx context.Context, url string, timeout time.Duration, check func(context.Context, string) error) error {
	if check == nil {
		check = defaultHTTPHealthCheck
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := check(ctx, url); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			if lastErr != nil {
				return fmt.Errorf("health check timeout for %s: %w", url, lastErr)
			}
			return fmt.Errorf("health check timeout for %s", url)
		case <-ticker.C:
		}
	}
}

func parseDevRestartArgs(args []string) (string, bool, time.Duration, error) {
	waitReady := true
	timeout := 60 * time.Second
	service := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-wait":
			waitReady = false
		case "--timeout":
			if i+1 >= len(args) {
				return "", false, 0, fmt.Errorf("--timeout requires seconds")
			}
			secs, err := strconv.Atoi(args[i+1])
			if err != nil || secs <= 0 {
				return "", false, 0, fmt.Errorf("--timeout must be positive integer seconds")
			}
			timeout = time.Duration(secs) * time.Second
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", false, 0, fmt.Errorf("unsupported dev restart flag %q", args[i])
			}
			if service != "" {
				return "", false, 0, fmt.Errorf("dev restart accepts a single service")
			}
			service = args[i]
		}
	}
	if service == "" {
		return "", false, 0, fmt.Errorf("dev restart requires <service>")
	}
	return service, waitReady, timeout, nil
}

func runDevRestartContext(ctx context.Context, args []string, providerCfg provider.Provider, projectCfg config.Project, root string, rawEnv []string, stdout, stderr io.Writer, runner func(context.Context, process.Request) process.Result, httpCheck func(context.Context, string) error) int {
	requested, waitReady, timeout, err := parseDevRestartArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if runner == nil {
		runner = process.Run
	}
	serviceName, svc := resolveDevService(projectCfg, requested)
	project := devProjectName(environMap(rawEnv), projectCfg)
	files := devComposeFiles(root)
	childEnv := withDevProjectEnv(rawEnv, root, project, projectCfg.Namespace)

	restartArgs, err := devComposeArgsForProject(providerCfg.Compose, project, files, "restart", serviceName, nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	res := runner(ctx, process.Request{Args: restartArgs, Dir: root, Env: childEnv, Stdout: stdout, Stderr: stderr})
	if res.ExitCode != 0 {
		if res.Err != nil {
			fmt.Fprintln(stderr, res.Err)
		}
		return res.ExitCode
	}
	if !waitReady {
		return 0
	}

	psArgs := append([]string{}, providerCfg.Compose...)
	psArgs = append(psArgs, "-p", project)
	for _, file := range files {
		psArgs = append(psArgs, "-f", file)
	}
	psArgs = append(psArgs, "ps", "--format", "json")
	if err := (lifecycle.Readiness{Run: runner, Dir: root, Env: childEnv}).Wait(ctx, psArgs, []string{serviceName}, timeout); err != nil {
		fmt.Fprintf(stderr, "dev restart readiness failed: %v\n", err)
		return 1
	}

	if len(svc.Ports) > 0 {
		healthPath := svc.HealthCheck
		if healthPath == "" {
			healthPath = "/health"
		}
		if !strings.HasPrefix(healthPath, "/") {
			healthPath = "/" + healthPath
		}
		url := fmt.Sprintf("http://127.0.0.1:%d%s", svc.Ports[0], healthPath)
		if err := waitHTTPHealthy(ctx, url, timeout, httpCheck); err != nil {
			fmt.Fprintf(stderr, "dev restart health check failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stderr, "Service %s restarted and ready.\n", serviceName)
	return 0
}

// ponytail: polling watch checks Go file mtimes every 500ms; switch to fsnotify if sub-100ms latency on large monorepos is needed.
func snapshotGoSourceFiles(dir string) map[string]int64 {
	state := map[string]int64{}
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == ".test_logs" || name == "vendor" || strings.HasPrefix(name, ".leedevkit") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" {
			if info, infoErr := entry.Info(); infoErr == nil {
				state[path] = info.ModTime().UnixNano()
			}
		}
		return nil
	})
	return state
}

func goSnapshotChanged(prev, next map[string]int64) bool {
	if len(prev) != len(next) {
		return true
	}
	for k, v := range next {
		if prevVal, ok := prev[k]; !ok || prevVal != v {
			return true
		}
	}
	return false
}

func runDevWatchContext(ctx context.Context, args []string, providerCfg provider.Provider, projectCfg config.Project, root string, rawEnv []string, pollInterval time.Duration, stdout, stderr io.Writer, runner func(context.Context, process.Request) process.Result, httpCheck func(context.Context, string) error) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "usage: leedevkit dev watch [service]")
		return 2
	}
	requested := ""
	if len(args) == 1 {
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprintf(stderr, "unsupported dev watch flag %q\n", args[0])
			return 2
		}
		requested = args[0]
	} else {
		requested = configuredServiceName(projectCfg, "go")
	}
	serviceName, svc := resolveDevService(projectCfg, requested)
	watchDir := root
	if svc.Workdir != "" {
		watchDir = filepath.Join(root, filepath.FromSlash(svc.Workdir))
	}
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	fmt.Fprintf(stderr, "Watching Go sources in %s for service %s...\n", watchDir, serviceName)
	prev := snapshotGoSourceFiles(watchDir)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
			curr := snapshotGoSourceFiles(watchDir)
			if goSnapshotChanged(prev, curr) {
				prev = curr
				fmt.Fprintf(stderr, "Change detected; restarting %s...\n", serviceName)
				_ = runDevRestartContext(ctx, []string{serviceName}, providerCfg, projectCfg, root, rawEnv, stdout, stderr, runner, httpCheck)
			}
		}
	}
}

type devMigrateOptions struct {
	Action    string
	Check     bool
	Baseline  string
	Dir       string
	DBService string
	Help      bool
}

func parseDevMigrateArgs(args []string) (devMigrateOptions, error) {
	opts := devMigrateOptions{Action: "up"}
	actionSet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			opts.Help = true
			return opts, nil
		case "up", "status":
			if actionSet {
				return devMigrateOptions{}, fmt.Errorf("multiple dev migrate actions specified")
			}
			opts.Action = args[i]
			actionSet = true
		case "--check":
			opts.Check = true
			if !actionSet {
				opts.Action = "status"
			}
		case "--baseline":
			if i+1 >= len(args) {
				return devMigrateOptions{}, fmt.Errorf("--baseline requires a migration filename")
			}
			opts.Baseline = args[i+1]
			i++
		case "--dir":
			if i+1 >= len(args) {
				return devMigrateOptions{}, fmt.Errorf("--dir requires a directory path")
			}
			opts.Dir = args[i+1]
			i++
		case "--db-service":
			if i+1 >= len(args) {
				return devMigrateOptions{}, fmt.Errorf("--db-service requires a compose service name")
			}
			opts.DBService = args[i+1]
			i++
		default:
			return devMigrateOptions{}, fmt.Errorf("unsupported dev migrate argument %q", args[i])
		}
	}
	return opts, nil
}

type sqlMigration struct {
	Version  string
	Path     string
	Checksum string
}

func discoverMigrationsDir(root string, projectCfg config.Project, explicitDir string) (string, error) {
	if explicitDir != "" {
		cleaned := filepath.Clean(explicitDir)
		if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("migrations dir %q must stay inside project", explicitDir)
		}
		full := filepath.Join(root, cleaned)
		if info, err := os.Stat(full); err != nil || !info.IsDir() {
			return "", fmt.Errorf("migrations directory not found: %s", explicitDir)
		}
		return full, nil
	}
	candidates := []string{}
	if projectCfg.DevMigrate.Dir != "" {
		candidates = append(candidates, projectCfg.DevMigrate.Dir)
	}
	candidates = append(candidates, "migrations")
	if goSvc, ok := projectCfg.ServiceForLanguage("go"); ok && goSvc.Workdir != "" {
		candidates = append(candidates, filepath.Join(goSvc.Workdir, "migrations"), filepath.Join(goSvc.Workdir, "db", "migrations"))
	}
	candidates = append(candidates, filepath.Join("db", "migrations"), filepath.Join("sql", "migrations"))
	for _, rel := range candidates {
		cleaned := filepath.Clean(rel)
		if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			continue
		}
		full := filepath.Join(root, cleaned)
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			return full, nil
		}
	}
	return "", fmt.Errorf("no migrations directory found (configure [dev.migrate] in leedevkit.toml or pass --dir)")
}

func loadSQLMigrations(dir string) ([]sqlMigration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var migrations []sqlMigration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".sql") || strings.HasSuffix(lower, ".down.sql") || strings.HasSuffix(lower, "_down.sql") {
			continue
		}
		if !safeMigrationName.MatchString(name) {
			return nil, fmt.Errorf("unsafe migration filename %q", name)
		}
		fullPath := filepath.Join(dir, name)
		info, err := os.Lstat(fullPath)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("migration %q must be a regular file", name)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		migrations = append(migrations, sqlMigration{
			Version:  name,
			Path:     fullPath,
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func buildDevPsqlArgs(compose []string, project string, files []string, dbService, dbUser, dbName string, extraPsqlArgs ...string) []string {
	args := append([]string{}, compose...)
	args = append(args, "-p", project)
	for _, file := range files {
		args = append(args, "-f", file)
	}
	script := `user="${LEEDEVKIT_DB_USER:-${POSTGRES_USER:-postgres}}"; db="${LEEDEVKIT_DB_NAME:-${POSTGRES_DB:-$user}}"; exec psql -X -v ON_ERROR_STOP=1 -U "$user" -d "$db" "$@"`
	args = append(args, "exec", "-T")
	if dbUser != "" {
		args = append(args, "-e", "LEEDEVKIT_DB_USER="+dbUser)
	}
	if dbName != "" {
		args = append(args, "-e", "LEEDEVKIT_DB_NAME="+dbName)
	}
	args = append(args, dbService, "sh", "-c", script, "psql")
	args = append(args, extraPsqlArgs...)
	return args
}

func resolveDevDBService(ctx context.Context, compose []string, project string, files []string, root string, env []string, configured string, runner func(context.Context, process.Request) process.Result) string {
	if configured != "" {
		return configured
	}
	for _, candidate := range []string{"postgres", "db"} {
		if isComposeServiceReady(ctx, compose, project, files, candidate, root, env, runner) {
			return candidate
		}
	}
	return "postgres"
}

func runDevMigrateContext(ctx context.Context, args []string, providerCfg provider.Provider, projectCfg config.Project, root string, rawEnv []string, stdout, stderr io.Writer, runner func(context.Context, process.Request) process.Result) int {
	opts, err := parseDevMigrateArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if opts.Help {
		fmt.Fprintln(stdout, "Usage: leedevkit dev migrate [up|status] [--check] [--baseline <file.sql>] [--dir <path>] [--db-service <svc>]")
		return 0
	}
	if runner == nil {
		runner = process.Run
	}
	project := devProjectName(environMap(rawEnv), projectCfg)
	files := devComposeFiles(root)
	childEnv := withDevProjectEnv(rawEnv, root, project, projectCfg.Namespace)

	// Custom migration command configured in [dev.migrate].
	if len(projectCfg.DevMigrate.Command) > 0 || len(projectCfg.DevMigrate.StatusCommand) > 0 {
		svcName := projectCfg.DevMigrate.Service
		if svcName == "" {
			svcName = configuredServiceName(projectCfg, "go")
		}
		resolvedName, svc := resolveDevService(projectCfg, svcName)
		workdir, err := resolveServiceWorkdir(root, resolvedName, svc, "")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		cmd := projectCfg.DevMigrate.Command
		if opts.Action == "status" {
			cmd = projectCfg.DevMigrate.StatusCommand
			if len(cmd) == 0 {
				fmt.Fprintln(stderr, "dev.migrate.status_command is not configured in leedevkit.toml")
				return 2
			}
		} else if len(cmd) == 0 {
			fmt.Fprintln(stderr, "dev.migrate.command is not configured in leedevkit.toml")
			return 2
		}
		execArgs := append([]string{}, providerCfg.Compose...)
		execArgs = append(execArgs, "-p", project)
		for _, file := range files {
			execArgs = append(execArgs, "-f", file)
		}
		execArgs = append(execArgs, "exec", "-T", "-w", workdir, resolvedName)
		execArgs = append(execArgs, cmd...)
		res := runner(ctx, process.Request{Args: execArgs, Dir: root, Env: childEnv, Stdout: stdout, Stderr: stderr})
		return res.ExitCode
	}

	// Built-in incremental SQL migration & drift check against dev Postgres.
	migrationsDir, err := discoverMigrationsDir(root, projectCfg, opts.Dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	migrations, err := loadSQLMigrations(migrationsDir)
	if err != nil {
		fmt.Fprintf(stderr, "load migrations: %v\n", err)
		return 2
	}
	dbServiceCfg := opts.DBService
	if dbServiceCfg == "" {
		dbServiceCfg = projectCfg.DevMigrate.DBService
	}
	dbService := resolveDevDBService(ctx, providerCfg.Compose, project, files, root, childEnv, dbServiceCfg, runner)
	dbUser := projectCfg.DevMigrate.DBUser
	dbName := projectCfg.DevMigrate.DBName

	initSQL := `CREATE TABLE IF NOT EXISTS _leedevkit_migrations (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()); SELECT version || '|' || checksum FROM _leedevkit_migrations ORDER BY version;`
	var queryOut, queryErr bytes.Buffer
	queryArgs := buildDevPsqlArgs(providerCfg.Compose, project, files, dbService, dbUser, dbName, "-t", "-A", "-c", initSQL)
	queryRes := runner(ctx, process.Request{Args: queryArgs, Dir: root, Env: childEnv, Stdout: &queryOut, Stderr: &queryErr})
	if queryRes.ExitCode != 0 {
		errMsg := strings.TrimSpace(queryErr.String())
		if errMsg == "" {
			errMsg = queryRes.Diagnostic()
		}
		fmt.Fprintf(stderr, "dev migrate failed to query dev database (%s): %s\n", dbService, process.RedactDiagnostic(errMsg))
		return 1
	}
	rawOut := queryRes.Stdout
	if len(rawOut) == 0 {
		rawOut = queryOut.Bytes()
	}
	applied := map[string]string{}
	for _, line := range strings.Split(string(rawOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "CREATE TABLE") {
			continue
		}
		ver, sum, ok := strings.Cut(line, "|")
		if ok {
			applied[strings.TrimSpace(ver)] = strings.TrimSpace(sum)
		}
	}

	if opts.Baseline != "" {
		foundBaseline := false
		var baselineSQL strings.Builder
		for _, m := range migrations {
			if m.Version <= opts.Baseline {
				if m.Version == opts.Baseline {
					foundBaseline = true
				}
				if _, exists := applied[m.Version]; !exists {
					fmt.Fprintf(&baselineSQL, "INSERT INTO _leedevkit_migrations (version, checksum) VALUES ('%s', '%s') ON CONFLICT (version) DO NOTHING;\n", m.Version, m.Checksum)
					applied[m.Version] = m.Checksum
				}
			}
		}
		if !foundBaseline {
			fmt.Fprintf(stderr, "baseline migration %q not found in %s\n", opts.Baseline, migrationsDir)
			return 2
		}
		if baselineSQL.Len() > 0 {
			baseArgs := buildDevPsqlArgs(providerCfg.Compose, project, files, dbService, dbUser, dbName, "-q", "-c", baselineSQL.String())
			baseRes := runner(ctx, process.Request{Args: baseArgs, Dir: root, Env: childEnv, Stdout: stdout, Stderr: stderr})
			if baseRes.ExitCode != 0 {
				return 1
			}
		}
	}

	diskMap := make(map[string]sqlMigration, len(migrations))
	for _, m := range migrations {
		diskMap[m.Version] = m
	}
	var driftErrors []string
	appliedNames := make([]string, 0, len(applied))
	for ver := range applied {
		appliedNames = append(appliedNames, ver)
	}
	sort.Strings(appliedNames)
	for _, ver := range appliedNames {
		disk, ok := diskMap[ver]
		if !ok {
			driftErrors = append(driftErrors, fmt.Sprintf("drift: applied migration %q is missing on disk", ver))
			continue
		}
		if disk.Checksum != applied[ver] {
			driftErrors = append(driftErrors, fmt.Sprintf("drift: migration %q checksum changed (db=%s disk=%s)", ver, applied[ver][:12], disk.Checksum[:12]))
		}
	}
	if len(driftErrors) > 0 {
		for _, d := range driftErrors {
			fmt.Fprintln(stderr, d)
		}
		return 1
	}

	var pending []sqlMigration
	for _, m := range migrations {
		if _, ok := applied[m.Version]; !ok {
			pending = append(pending, m)
		}
	}

	if opts.Action == "status" {
		for _, m := range migrations {
			state := "pending"
			if _, ok := applied[m.Version]; ok {
				state = "applied"
			}
			fmt.Fprintf(stdout, "%s\t%s\n", state, m.Version)
		}
		fmt.Fprintf(stderr, "Migrations: %d applied, %d pending, 0 drifted.\n", len(applied), len(pending))
		if opts.Check && len(pending) > 0 {
			return 1
		}
		return 0
	}

	if len(pending) == 0 {
		fmt.Fprintf(stderr, "Dev database up to date (%d applied, 0 pending).\n", len(applied))
		return 0
	}

	for _, m := range pending {
		sqlBytes, readErr := os.ReadFile(m.Path)
		if readErr != nil {
			fmt.Fprintf(stderr, "read migration %s: %v\n", m.Version, readErr)
			return 1
		}
		applyArgs := buildDevPsqlArgs(providerCfg.Compose, project, files, dbService, dbUser, dbName)
		applyRes := runner(ctx, process.Request{
			Args:   applyArgs,
			Dir:    root,
			Env:    childEnv,
			Stdin:  bytes.NewReader(sqlBytes),
			Stdout: stdout,
			Stderr: stderr,
		})
		if applyRes.ExitCode != 0 {
			fmt.Fprintf(stderr, "migration %s failed\n", m.Version)
			return applyRes.ExitCode
		}
		recordSQL := fmt.Sprintf("INSERT INTO _leedevkit_migrations (version, checksum) VALUES ('%s', '%s');", m.Version, m.Checksum)
		recArgs := buildDevPsqlArgs(providerCfg.Compose, project, files, dbService, dbUser, dbName, "-q", "-c", recordSQL)
		recRes := runner(ctx, process.Request{Args: recArgs, Dir: root, Env: childEnv, Stdout: io.Discard, Stderr: stderr})
		if recRes.ExitCode != 0 {
			fmt.Fprintf(stderr, "failed to record migration %s\n", m.Version)
			return 1
		}
		fmt.Fprintf(stderr, "Applied migration %s\n", m.Version)
	}
	fmt.Fprintf(stderr, "Applied %d incremental migration(s).\n", len(pending))
	return 0
}

func runDevCLI(args []string, root string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[1] == "--help" || args[1] == "-h" || args[1] == "help" {
		if len(args) >= 2 {
			fmt.Fprint(stdout, devUsage)
			return 0
		}
		fmt.Fprintln(stderr, "unsupported dev command")
		return 2
	}
	sub := args[1]
	switch sub {
	case "up", "down", "ps", "clean", "logs", "exec", "restart", "watch", "migrate":
	default:
		fmt.Fprintln(stderr, "unsupported dev command")
		return 2
	}
	envMap := environMap(os.Environ())
	providerConfig, err := resolveProviderForProject(root, envMap)
	if err != nil {
		fmt.Fprintf(stderr, "invalid provider: %v\n", err)
		return 2
	}
	projectConfig, cfgErr := config.Load(filepath.Join(root, "leedevkit.toml"))
	if cfgErr != nil {
		fmt.Fprintf(stderr, "invalid project config: %v\n", cfgErr)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch sub {
	case "up", "down":
		return runDev(sub, providerConfig, root, stdout, stderr, process.Run)
	case "restart":
		return runDevRestartContext(ctx, args[2:], providerConfig, projectConfig, root, os.Environ(), stdout, stderr, process.Run, nil)
	case "watch":
		return runDevWatchContext(ctx, args[2:], providerConfig, projectConfig, root, os.Environ(), 500*time.Millisecond, stdout, stderr, process.Run, nil)
	case "migrate":
		return runDevMigrateContext(ctx, args[2:], providerConfig, projectConfig, root, os.Environ(), stdout, stderr, process.Run)
	}

	service := ""
	var command []string
	if sub == "logs" && len(args) > 2 {
		service = args[2]
	}
	if sub == "exec" {
		if len(args) < 3 {
			fmt.Fprintln(stderr, "dev exec requires service")
			return 2
		}
		service = args[2]
		command = args[3:]
	}
	project := devProjectName(envMap, projectConfig)
	argv, err := devComposeArgsForProject(providerConfig.Compose, project, devComposeFiles(root), sub, service, command)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	childEnv := withDevProjectEnv(os.Environ(), root, project, projectConfig.Namespace)
	result := process.Run(ctx, process.Request{Args: argv, Dir: root, Env: childEnv, Stdin: stdin, Stdout: stdout, Stderr: stderr})
	return result.ExitCode
}
