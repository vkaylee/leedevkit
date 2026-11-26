package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestDoctorAssetRootForInstalledBinary(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "bin", "go", "leedevkit-go")
	if got := doctorAssetRootFromExecutable(binary); got != root {
		t.Fatalf("asset root = %q, want %q", got, root)
	}
}

func TestGlobalDoctorAssetRootUsesExecutableDirectory(t *testing.T) {
	assetRoot, err := globalDoctorAssetRoot()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Dir(filepath.Dir(resolved))
	if assetRoot != want {
		t.Fatalf("global asset root = %q, want %q", assetRoot, want)
	}
}

func TestRunDoctorReadOnlyReportsProjectAndGlobalState(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[project]\nname = \"test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent", "rules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent", "rules", "coding.md"), []byte("# coding"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(assets, ".agent", "rules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "VERSION"), []byte("0.11.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if code := runDoctorWithAssets(root, assets, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("doctor exit code = %d, output = %q", code, stderr.String())
	}
	for _, want := range []string{"Project config:", ".agent: real directory, 1 rulebooks", "Global DevKit:", "Global DevKit assets: available", "runtime docker:"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("doctor output missing %q: %s", want, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), filepath.Join(root, ".leedevkit")) {
		t.Fatalf("doctor inspected project runtime instead of global assets: %s", stderr.String())
	}
}

func TestRunDoctorWarnsAboutLegacyLauncherWithoutRemoving(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	launcher := filepath.Join(root, "leedevkit")
	if err := os.WriteFile(launcher, []byte("legacy"), 0o700); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	runDoctorWithAssets(root, assets, &bytes.Buffer{}, &stderr)
	if !strings.Contains(stderr.String(), "legacy project launcher detected and left unchanged") {
		t.Fatalf("legacy launcher warning missing: %s", stderr.String())
	}
	if got, err := os.ReadFile(launcher); err != nil || string(got) != "legacy" {
		t.Fatalf("legacy launcher changed: %q, %v", got, err)
	}
}

func TestRunDoctorDoesNotModifyProjectData(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit"), 0o700); err != nil {
		t.Fatal(err)
	}
	projectData := filepath.Join(root, ".leedevkit", "VERSION")
	if err := os.WriteFile(projectData, []byte("project-version"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	runDoctorWithAssets(root, assets, &bytes.Buffer{}, &stderr)
	if got, err := os.ReadFile(projectData); err != nil || string(got) != "project-version" {
		t.Fatalf("project runtime data changed: %q, %v", got, err)
	}
}

func TestRepairDoctorRulesCopiesGlobalRulesAndPreservesProjectRules(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	globalRules := filepath.Join(assets, ".agent", "rules")
	projectRules := filepath.Join(root, ".agent", "rules")
	if err := os.MkdirAll(globalRules, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectRules, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalRules, "base.md"), []byte("base"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRules, "base.md"), []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalRules, "testing.md"), []byte("testing"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := repairDoctorRulesFromAssets(root, assets); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(projectRules, "base.md")); err != nil || string(got) != "user" {
		t.Fatalf("user rule changed: %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(projectRules, "testing.md")); err != nil || string(got) != "testing" {
		t.Fatalf("missing global rule not copied: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".leedevkit")); !os.IsNotExist(err) {
		t.Fatalf("repair created project runtime data: %v", err)
	}
}

func TestRepairDoctorRulesRejectsSymlinkedProjectRules(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	globalRules := filepath.Join(assets, ".agent", "rules")
	if err := os.MkdirAll(globalRules, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(globalRules, filepath.Join(root, ".agent", "rules")); err != nil {
		t.Fatal(err)
	}
	if err := repairDoctorRulesFromAssets(root, assets); err == nil {
		t.Fatal("symlinked project rules accepted")
	}
}

func TestSweepDoctorStaleProjectsReportsCleanedAndErrors(t *testing.T) {
	var stderr bytes.Buffer
	res := sweepDoctorStaleProjectsWithRunner(context.Background(), []string{"docker"}, func(_ context.Context, request process.Request) process.Result {
		joined := strings.Join(request.Args, " ")
		switch {
		case strings.Contains(joined, "--format"):
			_, _ = request.Stdout.Write([]byte("leedevkit-test-old|2020-01-01T00:00:00Z\nleedevkit-dev|2020-01-01T00:00:00Z\n"))
			return process.Result{}
		case strings.Contains(joined, "ps -a -q"):
			_, _ = request.Stdout.Write([]byte("c1\n"))
			return process.Result{}
		default:
			return process.Result{}
		}
	}, &stderr)
	if res.Cleaned != 1 || len(res.Errors) != 0 || !strings.Contains(stderr.String(), "Cleaned 1 stale test/infra project(s).") {
		t.Fatalf("res = %#v, stderr = %q", res, stderr.String())
	}

	stderr.Reset()
	errRes := sweepDoctorStaleProjectsWithRunner(context.Background(), []string{"docker"}, func(_ context.Context, _ process.Request) process.Result {
		return process.Result{ExitCode: 1, Stderr: []byte("offline")}
	}, &stderr)
	if len(errRes.Errors) != 1 || !strings.Contains(stderr.String(), "warning: stale project sweep:") {
		t.Fatalf("errRes = %#v, stderr = %q", errRes, stderr.String())
	}
}

func TestDoctorWarnsShadowingArtifactsLegacyOrchestratorAndFixMigrates(t *testing.T) {
	root, assets := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(assets, ".agent", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "VERSION"), []byte("0.13.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
namespace = "acme"
[services.apiserver]
lang = "go"
[services.webdashboard]
lang = "typescript"
`
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "leedevkit"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit.bak"), 0o755); err != nil {
		t.Fatal(err)
	}
	dupClaude := "## LeeDevKit base context\n`templates/CLAUDE.base.md`.\n\n## LeeDevKit base context\n`templates/CLAUDE.base.md`.\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(dupClaude), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	npmProxy := filepath.Join(root, "scripts", "_npm.sh")
	if err := os.WriteFile(npmProxy, []byte("#!/bin/sh\nexec python3 .leedevkit/scripts/_orchestrator.py npm \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	runDoctorWithAssets(root, assets, &bytes.Buffer{}, &stderr)
	out := stderr.String()
	for _, want := range []string{
		"shadows global install",
		"legacy local .leedevkit/ directory detected",
		"legacy .leedevkit.bak/ artifact detected",
		"CLAUDE.md has duplicate or legacy LeeDevKit context markers",
		"legacy proxy script references removed _orchestrator.py",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}

	// Now verify repairDoctorProjectArtifacts (--fix) migrates _npm.sh and fixes CLAUDE.md idempotently!
	stderr.Reset()
	if err := repairDoctorProjectArtifacts(root, &stderr); err != nil {
		t.Fatalf("repairDoctorProjectArtifacts: %v", err)
	}
	migrated, err := os.ReadFile(npmProxy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(migrated), "_orchestrator.py") || !strings.Contains(string(migrated), "exec leedevkit exec webdashboard --") {
		t.Fatalf("unexpected migrated _npm.sh:\n%s", migrated)
	}
	fixedClaude, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(fixedClaude), "<!-- leedevkit:start -->") != 1 || strings.Count(string(fixedClaude), "## LeeDevKit base context") != 1 {
		t.Fatalf("unexpected repaired CLAUDE.md:\n%s", fixedClaude)
	}
}

func TestPruneDoctorTestVolumesAndCLIFlags(t *testing.T) {
	var stderr bytes.Buffer
	res := pruneDoctorTestVolumesWithRunner(context.Background(), []string{"docker"}, func(_ context.Context, req process.Request) process.Result {
		joined := strings.Join(req.Args, " ")
		if strings.Contains(joined, "volume ls") {
			return process.Result{Stdout: []byte("leedevkit-test-abc_shared-data\nleedevkit_go-module-cache\n")}
		}
		return process.Result{Stdout: []byte("leedevkit-test-abc_shared-data\n")}
	}, &stderr)
	if res.Removed != 1 || !strings.Contains(stderr.String(), "Pruned 1 stale test/infra volume(s).") {
		t.Fatalf("res = %#v, stderr = %q", res, stderr.String())
	}

	stderr.Reset()
	none := pruneDoctorTestVolumesWithRunner(context.Background(), []string{"docker"}, func(_ context.Context, _ process.Request) process.Result {
		return process.Result{Stdout: []byte("leedevkit_go-module-cache\n")}
	}, &stderr)
	if none.Removed != 0 || !strings.Contains(stderr.String(), "No stale test/infra volumes to prune.") {
		t.Fatalf("none = %#v, stderr = %q", none, stderr.String())
	}

	stderr.Reset()
	errRes := pruneDoctorTestVolumesWithRunner(context.Background(), []string{"docker"}, func(_ context.Context, _ process.Request) process.Result {
		return process.Result{ExitCode: 1, Stderr: []byte("offline")}
	}, &stderr)
	if len(errRes.Errors) != 1 || !strings.Contains(stderr.String(), "warning: test volume prune:") {
		t.Fatalf("errRes = %#v, stderr = %q", errRes, stderr.String())
	}

	var stdout bytes.Buffer
	stderr.Reset()
	if code := runDoctorCLI([]string{"doctor", "--help"}, t.TempDir(), &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "--prune-volumes") {
		t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stderr.Reset()
	if code := runDoctorCLI([]string{"doctor", "--unknown"}, t.TempDir(), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "unsupported doctor flag") {
		t.Fatalf("unknown flag code=%d stderr=%q", code, stderr.String())
	}
}
