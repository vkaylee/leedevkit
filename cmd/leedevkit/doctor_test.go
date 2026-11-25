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
