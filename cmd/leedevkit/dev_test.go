package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

func TestDevComposeArgsPreservesLifecycleBoundary(t *testing.T) {
	got, err := devComposeArgs([]string{"docker", "compose"}, "up")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "-p", "leedevkit-dev", "-f", "docker-compose.yml", "-f", ".compose/docker-compose.dev.yml", "up", "-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestDevComposeArgsSupportsReadOnlyOperations(t *testing.T) {
	for _, operation := range []string{"ps", "logs"} {
		args, err := devComposeArgs([]string{"compose"}, operation)
		if err != nil {
			t.Fatalf("%s args = %#v, err = %v", operation, args, err)
		}
		if operation == "logs" && args[len(args)-1] != "-f" {
			t.Fatalf("logs args = %#v", args)
		}
		if operation != "logs" && args[len(args)-1] != operation {
			t.Fatalf("%s args = %#v", operation, args)
		}
	}
}

func TestDevComposeArgsMapsCleanToVolumeTeardown(t *testing.T) {
	got, err := devComposeArgs([]string{"compose"}, "clean")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "-p", "leedevkit-dev", "-f", "docker-compose.yml", "-f", ".compose/docker-compose.dev.yml", "down", "-v"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clean args = %#v, want %#v", got, want)
	}
}

func TestDevComposeArgsRejectsExecWithoutService(t *testing.T) {
	if _, err := devComposeArgsWithOptions([]string{"compose"}, "exec", "", nil); err == nil {
		t.Fatal("exec without service accepted")
	}
}

func TestRunDevPropagatesComposeResult(t *testing.T) {
	var got []string
	code := runDev("down", provider.Provider{Compose: []string{"compose"}}, "/project", nil, nil, func(_ context.Context, request process.Request) process.Result {
		got = request.Args
		return process.Result{ExitCode: 7}
	})
	if code != 7 || len(got) == 0 || got[len(got)-1] != "down" {
		t.Fatalf("code=%d args=%#v", code, got)
	}
}

func TestDevRestartWaitsForComposeAndHealthEndpoint(t *testing.T) {
	projectCfg := config.Project{
		Namespace: "acme",
		Services: map[string]config.Service{
			"apiserver": {Name: "apiserver", Language: "go", Ports: []int{8080}, HealthCheck: "/api/v1/health"},
		},
		Targets: map[string][]string{"api": {"apiserver"}},
	}
	var composeCalls []string
	var healthChecks []string
	attempts := 0
	var stdout, stderr bytes.Buffer
	code := runDevRestartContext(
		context.Background(),
		[]string{"api", "--timeout", "2"},
		provider.Provider{Compose: []string{"docker", "compose"}},
		projectCfg,
		t.TempDir(),
		nil,
		&stdout,
		&stderr,
		func(_ context.Context, req process.Request) process.Result {
			joined := strings.Join(req.Args, " ")
			composeCalls = append(composeCalls, joined)
			if strings.Contains(joined, " ps --format json") {
				return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"apiserver","State":"running","Health":"healthy"}]`)}
			}
			return process.Result{ExitCode: 0}
		},
		func(_ context.Context, url string) error {
			healthChecks = append(healthChecks, url)
			attempts++
			if attempts < 2 {
				return errors.New("connection refused")
			}
			return nil
		},
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if len(composeCalls) < 2 || !strings.Contains(composeCalls[0], "-p acme-dev") || !strings.HasSuffix(composeCalls[0], "restart apiserver") {
		t.Fatalf("unexpected composeCalls: %v", composeCalls)
	}
	if len(healthChecks) != 2 || healthChecks[0] != "http://127.0.0.1:8080/api/v1/health" {
		t.Fatalf("unexpected healthChecks: %v", healthChecks)
	}
}

func TestDevWatchDetectsGoFileChangesAndRestarts(t *testing.T) {
	root := t.TempDir()
	apiDir := filepath.Join(root, "apiserver")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(apiDir, "main.go")
	if err := os.WriteFile(srcFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectCfg := config.Project{
		Namespace: "acme",
		Services: map[string]config.Service{
			"apiserver": {Name: "apiserver", Language: "go", Workdir: "apiserver", Ports: []int{8080}},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restarted := make(chan struct{}, 1)
	go func() {
		time.Sleep(30 * time.Millisecond)
		now := time.Now().Add(2 * time.Second)
		_ = os.WriteFile(srcFile, []byte("package main\nfunc init() {}\n"), 0o644)
		_ = os.Chtimes(srcFile, now, now)
	}()
	var stdout, stderr bytes.Buffer
	code := runDevWatchContext(
		ctx,
		[]string{"apiserver"},
		provider.Provider{Compose: []string{"compose"}},
		projectCfg,
		root,
		nil,
		15*time.Millisecond,
		&stdout,
		&stderr,
		func(_ context.Context, req process.Request) process.Result {
			joined := strings.Join(req.Args, " ")
			if strings.Contains(joined, "restart apiserver") {
				select {
				case restarted <- struct{}{}:
				default:
				}
				cancel()
			}
			if strings.Contains(joined, "ps --format json") {
				return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"apiserver","State":"running"}]`)}
			}
			return process.Result{ExitCode: 0}
		},
		func(context.Context, string) error { return nil },
	)
	if code != 0 {
		t.Fatalf("watch code=%d stderr=%q", code, stderr.String())
	}
	select {
	case <-restarted:
	default:
		t.Fatalf("expected restart after Go file edit; stderr=%q", stderr.String())
	}
}

func TestDevMigrateIncrementalSQLBaselineDriftAndCustomCommand(t *testing.T) {
	root := t.TempDir()
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sql1 := "CREATE TABLE users (id INT PRIMARY KEY);\n"
	sql2 := "ALTER TABLE users ADD COLUMN email TEXT;\n"
	if err := os.WriteFile(filepath.Join(migDir, "001_init.sql"), []byte(sql1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migDir, "002_add_email.sql"), []byte(sql2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migDir, "002_add_email.down.sql"), []byte("DROP TABLE users;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum1Bytes := sha256.Sum256([]byte(sql1))
	sum1 := hex.EncodeToString(sum1Bytes[:])

	projectCfg := config.Project{Namespace: "acme"}
	prov := provider.Provider{Compose: []string{"docker", "compose"}}

	t.Run("applies only pending migration when 001 is already applied", func(t *testing.T) {
		var executedSQL []string
		var recorded []string
		var stdout, stderr bytes.Buffer
		code := runDevMigrateContext(
			context.Background(),
			nil,
			prov,
			projectCfg,
			root,
			nil,
			&stdout,
			&stderr,
			func(_ context.Context, req process.Request) process.Result {
				joined := strings.Join(req.Args, " ")
				switch {
				case strings.Contains(joined, "ps --format json"):
					return process.Result{ExitCode: 0, Stdout: []byte(`[{"Service":"postgres","State":"running","Health":"healthy"}]`)}
				case strings.Contains(joined, "SELECT version"):
					return process.Result{ExitCode: 0, Stdout: []byte("001_init.sql|" + sum1 + "\n")}
				case strings.Contains(joined, "INSERT INTO _leedevkit_migrations"):
					recorded = append(recorded, joined)
					return process.Result{ExitCode: 0}
				case req.Stdin != nil:
					buf, _ := os.ReadFile(filepath.Join(migDir, "002_add_email.sql"))
					var got bytes.Buffer
					_, _ = got.ReadFrom(req.Stdin)
					executedSQL = append(executedSQL, got.String())
					if got.String() != string(buf) {
						t.Errorf("unexpected stdin SQL: %q", got.String())
					}
					return process.Result{ExitCode: 0}
				default:
					return process.Result{ExitCode: 0}
				}
			},
		)
		if code != 0 || len(executedSQL) != 1 || len(recorded) != 1 || !strings.Contains(recorded[0], "002_add_email.sql") {
			t.Fatalf("code=%d executed=%v recorded=%v stderr=%q", code, executedSQL, recorded, stderr.String())
		}
	})

	t.Run("detects checksum drift and aborts before running pending migrations", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runDevMigrateContext(
			context.Background(),
			[]string{"status"},
			prov,
			projectCfg,
			root,
			nil,
			&stdout,
			&stderr,
			func(_ context.Context, req process.Request) process.Result {
				joined := strings.Join(req.Args, " ")
				if strings.Contains(joined, "SELECT version") {
					return process.Result{ExitCode: 0, Stdout: []byte("001_init.sql|" + strings.Repeat("0", 64) + "\n")}
				}
				return process.Result{ExitCode: 0}
			},
		)
		if code != 1 || !strings.Contains(stderr.String(), "drift: migration \"001_init.sql\" checksum changed") {
			t.Fatalf("expected drift failure, got code=%d stderr=%q", code, stderr.String())
		}
	})

	t.Run("custom [dev.migrate] command override", func(t *testing.T) {
		customCfg := config.Project{
			Namespace: "acme",
			Services:  map[string]config.Service{"apiserver": {Name: "apiserver", Language: "go", Workdir: "apiserver"}},
			DevMigrate: config.DevMigrate{
				Service:       "apiserver",
				Command:       []string{"go", "run", "./cmd/migrate", "up"},
				StatusCommand: []string{"go", "run", "./cmd/migrate", "status"},
			},
		}
		var gotArgs []string
		var stdout, stderr bytes.Buffer
		code := runDevMigrateContext(context.Background(), []string{"status"}, prov, customCfg, root, nil, &stdout, &stderr, func(_ context.Context, req process.Request) process.Result {
			gotArgs = req.Args
			return process.Result{ExitCode: 0}
		})
		if code != 0 || !strings.Contains(strings.Join(gotArgs, " "), "-p acme-dev") || !strings.Contains(strings.Join(gotArgs, " "), "exec -T -w /workspace/apiserver apiserver go run ./cmd/migrate status") {
			t.Fatalf("code=%d gotArgs=%v stderr=%q", code, gotArgs, stderr.String())
		}
	})
}
