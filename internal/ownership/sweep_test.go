package ownership

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestSweepOnlyCleansOldUnlockedProjects(t *testing.T) {
	now := time.Unix(1000, 0)
	projects := []SweepProject{
		{Name: "leedevkit-test-old", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-infra-old", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-test-young", Created: now.Add(-5 * time.Minute)},
		{Name: "leedevkit-infra-young", Created: now.Add(-5 * time.Minute)},
		{Name: "other-old", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-test-unknown"},
		{Name: "leedevkit-test-locked", Created: now.Add(-2 * time.Hour), Locked: true},
		{Name: "leedevkit-infra-locked", Created: now.Add(-2 * time.Hour), Locked: true},
	}
	var cleaned []string
	result := Sweep(projects, now, time.Hour, func(name string) error {
		cleaned = append(cleaned, name)
		return nil
	})
	if len(cleaned) != 2 || cleaned[0] != "leedevkit-test-old" || cleaned[1] != "leedevkit-infra-old" {
		t.Fatalf("cleaned = %#v", cleaned)
	}
	if result.Kept != 6 || result.Cleaned != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSweepContinuesAfterCleanupError(t *testing.T) {
	now := time.Unix(1000, 0)
	result := Sweep([]SweepProject{
		{Name: "leedevkit-test-a", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-test-b", Created: now.Add(-2 * time.Hour)},
	}, now, time.Hour, func(name string) error {
		if name == "leedevkit-test-a" {
			return errSweep
		}
		return nil
	})
	if result.Cleaned != 1 || len(result.Errors) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSweepRuntimeCleansStaleUnlockedProjectsWithoutVolumes(t *testing.T) {
	now := time.Date(2026, 11, 25, 14, 0, 0, 0, time.FixedZone("+07", 7*3600))
	var calls [][]string
	run := func(_ context.Context, request process.Request) process.Result {
		calls = append(calls, append([]string(nil), request.Args...))
		joined := strings.Join(request.Args, " ")
		switch {
		case strings.Contains(joined, `--format {{.Label "com.docker.compose.project"}}|{{.CreatedAt}}`):
			_, _ = request.Stdout.Write([]byte(strings.Join([]string{
				"leedevkit-dev|2026-11-25 10:00:00 +0700 +07",
				"leedevkit-test-stale|2026-11-25 11:00:00 +0700 +07",
				"leedevkit-test-stale|2026-11-25 11:05:00 +0700 +07",
				"leedevkit-infra-locked|2026-11-25 10:00:00 +0700 +07",
				"leedevkit-test-young|2026-11-25T13:50:00+07:00",
				"malformed-line",
			}, "\n")))
			return process.Result{}
		case strings.Contains(joined, "ps -a -q --filter label=com.docker.compose.project=leedevkit-test-stale"):
			_, _ = request.Stdout.Write([]byte("cid1\ncid2\n"))
			return process.Result{}
		case strings.Contains(joined, "network ls -q --filter label=com.docker.compose.project=leedevkit-test-stale"):
			_, _ = request.Stdout.Write([]byte("net1\n"))
			return process.Result{}
		default:
			return process.Result{}
		}
	}
	result := SweepRuntime(context.Background(), RuntimeSweepOptions{
		CLI:    []string{"docker"},
		Now:    now,
		MaxAge: time.Hour,
		Run:    run,
		IsLocked: func(project string) bool {
			return project == "leedevkit-infra-locked"
		},
	})
	if result.Cleaned != 1 || result.Kept != 2 || len(result.Errors) != 0 {
		t.Fatalf("result = %#v, calls = %#v", result, calls)
	}
	wantRM := []string{"docker", "rm", "-f", "cid1", "cid2"}
	if !reflect.DeepEqual(calls[2], wantRM) {
		t.Fatalf("rm call = %#v, want %#v", calls[2], wantRM)
	}
	wantNetRM := []string{"docker", "network", "rm", "net1"}
	if !reflect.DeepEqual(calls[4], wantNetRM) {
		t.Fatalf("network rm call = %#v, want %#v", calls[4], wantNetRM)
	}
	for _, call := range calls {
		if hasArg(call, "-v") || hasArg(call, "--volumes") {
			t.Fatalf("sweep passed volume removal flag: %#v", call)
		}
	}
}

func TestSweepRuntimeHandlesEmptyCLIAndErrors(t *testing.T) {
	if res := SweepRuntime(context.Background(), RuntimeSweepOptions{}); res.Cleaned != 0 || len(res.Errors) != 0 {
		t.Fatalf("empty cli result = %#v", res)
	}
	res := SweepRuntime(context.Background(), RuntimeSweepOptions{
		CLI: []string{"docker"},
		Run: func(_ context.Context, _ process.Request) process.Result {
			return process.Result{ExitCode: 1, Stderr: []byte("daemon offline")}
		},
	})
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Error(), "list compose containers") {
		t.Fatalf("discover error = %#v", res)
	}
	now := time.Date(2026, 11, 25, 14, 0, 0, 0, time.UTC)
	rmErr := SweepRuntime(context.Background(), RuntimeSweepOptions{
		CLI:    []string{"docker"},
		Now:    now,
		MaxAge: 0,
		Run: func(_ context.Context, request process.Request) process.Result {
			joined := strings.Join(request.Args, " ")
			switch {
			case strings.Contains(joined, "--format"):
				return process.Result{Stdout: []byte("leedevkit-infra-fail|2026-11-25T13:00:00Z\n")}
			case strings.Contains(joined, "ps -a -q"):
				return process.Result{Stdout: []byte("cid9\n")}
			case strings.Contains(joined, "rm -f"):
				return process.Result{ExitCode: 1, Stderr: []byte("rm failed")}
			default:
				return process.Result{}
			}
		},
		IsLocked: func(string) bool { return false },
	})
	if rmErr.Cleaned != 0 || len(rmErr.Errors) != 1 || !strings.Contains(rmErr.Errors[0].Error(), "remove project containers") {
		t.Fatalf("rm error = %#v", rmErr)
	}
}

func TestSweepRuntimeWorksWithRealProcessRun(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "fake-docker")
	logPath := filepath.Join(dir, "docker.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
case "$*" in
  *"--format"*) printf 'leedevkit-test-orphan|2020-01-01 00:00:00 +0000 UTC\n' ;;
  *"ps -a -q"*) printf 'cid-orphan\n' ;;
  *"network ls -q"*) printf 'net-orphan\n' ;;
esac
`
	if err := os.WriteFile(cli, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	res := SweepRuntime(context.Background(), RuntimeSweepOptions{
		CLI:      []string{cli},
		Now:      time.Date(2026, 11, 25, 14, 0, 0, 0, time.UTC),
		MaxAge:   time.Hour,
		IsLocked: func(string) bool { return false },
	})
	if res.Cleaned != 1 || len(res.Errors) != 0 {
		t.Fatalf("res = %#v", res)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), "rm -f cid-orphan") || !strings.Contains(string(logged), "network rm net-orphan") {
		t.Fatalf("unexpected docker calls:\n%s", logged)
	}
}

func TestPruneTestVolumesRemovesOnlyDanglingUnlockedManagedVolumes(t *testing.T) {
	if res := PruneTestVolumes(context.Background(), RuntimeSweepOptions{}); res.Removed != 0 || len(res.Errors) != 0 {
		t.Fatalf("empty cli result = %#v", res)
	}
	var calls [][]string
	res := PruneTestVolumes(context.Background(), RuntimeSweepOptions{
		CLI: []string{"docker"},
		IsLocked: func(project string) bool {
			return project == "leedevkit-test-locked"
		},
		Run: func(_ context.Context, request process.Request) process.Result {
			calls = append(calls, append([]string(nil), request.Args...))
			joined := strings.Join(request.Args, " ")
			switch {
			case strings.Contains(joined, "volume ls --filter dangling=true"):
				return process.Result{Stdout: []byte(strings.Join([]string{
					"leedevkit-test-abc_shared-data",
					"leedevkit-infra-123_tmp",
					"leedevkit-test-locked_shared-data",
					"leedevkit_go-module-cache",
					"leedevkit_cargo-registry",
					"leedevkit-dev_standalone_storage",
					"acme-test_db",
					"leedevkit-go-mod",
					"-bad-flag_vol",
				}, "\n"))}
			case strings.Contains(joined, "volume rm"):
				return process.Result{Stdout: []byte("leedevkit-infra-123_tmp\nleedevkit-test-abc_shared-data\n")}
			default:
				return process.Result{}
			}
		},
	})
	if res.Removed != 2 || res.Kept != 7 || len(res.Errors) != 0 {
		t.Fatalf("res = %#v, calls = %#v", res, calls)
	}
	wantRM := []string{"docker", "volume", "rm", "leedevkit-infra-123_tmp", "leedevkit-test-abc_shared-data"}
	if len(calls) != 2 || !reflect.DeepEqual(calls[1], wantRM) {
		t.Fatalf("calls = %#v, want rm %#v", calls, wantRM)
	}

	errRes := PruneTestVolumes(context.Background(), RuntimeSweepOptions{
		CLI: []string{"docker"},
		Run: func(_ context.Context, _ process.Request) process.Result {
			return process.Result{ExitCode: 1, Stderr: []byte("daemon offline")}
		},
	})
	if len(errRes.Errors) != 1 || !strings.Contains(errRes.Errors[0].Error(), "list dangling volumes") {
		t.Fatalf("errRes = %#v", errRes)
	}
}

func TestNamespaceIsolatedProjectsSweepAndPruneProtectDevAndSharedVolumes(t *testing.T) {
	now := time.Unix(10_000, 0)
	var swept []string
	res := sweepWithNamespace([]SweepProject{
		{Name: "acme-test-1a2b3c4d", Created: now.Add(-2 * time.Hour)},
		{Name: "acme-test", Created: now.Add(-2 * time.Hour)},
		{Name: "acme-dev", Created: now.Add(-2 * time.Hour)},
		{Name: "acme-test-locked", Created: now.Add(-2 * time.Hour), Locked: true},
	}, "acme", now, time.Hour, func(name string) error {
		swept = append(swept, name)
		return nil
	})
	if res.Cleaned != 2 || !reflect.DeepEqual(swept, []string{"acme-test-1a2b3c4d", "acme-test"}) {
		t.Fatalf("swept = %#v, res = %#v", swept, res)
	}

	var rmArgs []string
	pruneRes := PruneTestVolumes(context.Background(), RuntimeSweepOptions{
		CLI:       []string{"docker"},
		Namespace: "acme",
		IsLocked:  func(project string) bool { return project == "acme-test-locked" },
		Run: func(_ context.Context, request process.Request) process.Result {
			joined := strings.Join(request.Args, " ")
			switch {
			case strings.Contains(joined, "volume ls"):
				return process.Result{Stdout: []byte(strings.Join([]string{
					"acme-test-1a2b3c4d_pgdata",
					"acme-test-locked_pgdata",
					"acme-test_pgdata",
					"acme-dev_pgdata",
					"acme_shared_cache",
				}, "\n"))}
			case strings.Contains(joined, "volume rm"):
				rmArgs = append([]string(nil), request.Args...)
				return process.Result{Stdout: []byte("acme-test-1a2b3c4d_pgdata\n")}
			default:
				return process.Result{}
			}
		},
	})
	if pruneRes.Removed != 1 || pruneRes.Kept != 4 || !reflect.DeepEqual(rmArgs, []string{"docker", "volume", "rm", "acme-test-1a2b3c4d_pgdata"}) {
		t.Fatalf("pruneRes = %#v, rmArgs = %#v", pruneRes, rmArgs)
	}
}
