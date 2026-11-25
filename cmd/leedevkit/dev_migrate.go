package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

var safeMigrationName = regexp.MustCompile(`^[A-Za-z0-9_.-]+\.sql$`)

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
		if !config.IsRelativeWithinRoot(explicitDir) {
			return "", fmt.Errorf("migrations dir %q must stay inside project", explicitDir)
		}
		full := filepath.Join(root, filepath.Clean(explicitDir))
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
		if !config.IsRelativeWithinRoot(rel) {
			continue
		}
		full := filepath.Join(root, filepath.Clean(rel))
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
	args := composeBaseArgs(compose, project, files)
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

type devMigrateDB struct {
	compose []string
	project string
	files   []string
	service string
	user    string
	name    string
	root    string
	env     []string
	runner  func(context.Context, process.Request) process.Result
}

func (db devMigrateDB) psqlArgs(extra ...string) []string {
	return buildDevPsqlArgs(db.compose, db.project, db.files, db.service, db.user, db.name, extra...)
}

func runCustomDevMigrate(ctx context.Context, opts devMigrateOptions, providerCfg provider.Provider, projectCfg config.Project, root, project string, files, childEnv []string, stdout, stderr io.Writer, runner func(context.Context, process.Request) process.Result) int {
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
	execArgs := composeBaseArgs(providerCfg.Compose, project, files)
	execArgs = append(execArgs, "exec", "-T", "-w", workdir, resolvedName)
	execArgs = append(execArgs, cmd...)
	return runner(ctx, process.Request{Args: execArgs, Dir: root, Env: childEnv, Stdout: stdout, Stderr: stderr}).ExitCode
}

func queryAppliedMigrations(ctx context.Context, db devMigrateDB) (map[string]string, error) {
	initSQL := `CREATE TABLE IF NOT EXISTS _leedevkit_migrations (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()); SELECT version || '|' || checksum FROM _leedevkit_migrations ORDER BY version;`
	var queryOut, queryErr bytes.Buffer
	queryRes := db.runner(ctx, process.Request{Args: db.psqlArgs("-t", "-A", "-c", initSQL), Dir: db.root, Env: db.env, Stdout: &queryOut, Stderr: &queryErr})
	if queryRes.ExitCode != 0 {
		errMsg := strings.TrimSpace(queryErr.String())
		if errMsg == "" {
			errMsg = queryRes.Diagnostic()
		}
		return nil, fmt.Errorf("dev migrate failed to query dev database (%s): %s", db.service, process.RedactDiagnostic(errMsg))
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
		if ver, sum, ok := strings.Cut(line, "|"); ok {
			applied[strings.TrimSpace(ver)] = strings.TrimSpace(sum)
		}
	}
	return applied, nil
}

func checkMigrationDrift(migrations []sqlMigration, applied map[string]string) []string {
	diskMap := make(map[string]sqlMigration, len(migrations))
	for _, m := range migrations {
		diskMap[m.Version] = m
	}
	appliedNames := make([]string, 0, len(applied))
	for ver := range applied {
		appliedNames = append(appliedNames, ver)
	}
	sort.Strings(appliedNames)
	var driftErrors []string
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
	return driftErrors
}

func applyBaselineMigrations(ctx context.Context, db devMigrateDB, migrations []sqlMigration, applied map[string]string, baseline, migrationsDir string, stdout, stderr io.Writer) int {
	foundBaseline := false
	var baselineSQL strings.Builder
	for _, m := range migrations {
		if m.Version <= baseline {
			if m.Version == baseline {
				foundBaseline = true
			}
			if _, exists := applied[m.Version]; !exists {
				fmt.Fprintf(&baselineSQL, "INSERT INTO _leedevkit_migrations (version, checksum) VALUES ('%s', '%s') ON CONFLICT (version) DO NOTHING;\n", m.Version, m.Checksum)
				applied[m.Version] = m.Checksum
			}
		}
	}
	if !foundBaseline {
		fmt.Fprintf(stderr, "baseline migration %q not found in %s\n", baseline, migrationsDir)
		return 2
	}
	if baselineSQL.Len() > 0 {
		baseRes := db.runner(ctx, process.Request{Args: db.psqlArgs("-q", "-c", baselineSQL.String()), Dir: db.root, Env: db.env, Stdout: stdout, Stderr: stderr})
		if baseRes.ExitCode != 0 {
			return 1
		}
	}
	return 0
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

	if len(projectCfg.DevMigrate.Command) > 0 || len(projectCfg.DevMigrate.StatusCommand) > 0 {
		return runCustomDevMigrate(ctx, opts, providerCfg, projectCfg, root, project, files, childEnv, stdout, stderr, runner)
	}

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
	db := devMigrateDB{
		compose: providerCfg.Compose,
		project: project,
		files:   files,
		service: resolveDevDBService(ctx, providerCfg.Compose, project, files, root, childEnv, dbServiceCfg, runner),
		user:    projectCfg.DevMigrate.DBUser,
		name:    projectCfg.DevMigrate.DBName,
		root:    root,
		env:     childEnv,
		runner:  runner,
	}

	applied, err := queryAppliedMigrations(ctx, db)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if opts.Baseline != "" {
		if code := applyBaselineMigrations(ctx, db, migrations, applied, opts.Baseline, migrationsDir, stdout, stderr); code != 0 {
			return code
		}
	}
	if driftErrors := checkMigrationDrift(migrations, applied); len(driftErrors) > 0 {
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
		applyRes := runner(ctx, process.Request{
			Args:   db.psqlArgs(),
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
		recRes := runner(ctx, process.Request{Args: db.psqlArgs("-q", "-c", recordSQL), Dir: root, Env: childEnv, Stdout: io.Discard, Stderr: stderr})
		if recRes.ExitCode != 0 {
			fmt.Fprintf(stderr, "failed to record migration %s\n", m.Version)
			return 1
		}
		fmt.Fprintf(stderr, "Applied migration %s\n", m.Version)
	}
	fmt.Fprintf(stderr, "Applied %d incremental migration(s).\n", len(pending))
	return 0
}
