package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/ownership"
	"github.com/vkaylee/leedevkit/internal/process"
	runtimepkg "github.com/vkaylee/leedevkit/internal/runtime"
)

func globalDoctorAssetRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate leedevkit executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve leedevkit executable: %w", err)
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve leedevkit executable path: %w", err)
	}
	return doctorAssetRootFromExecutable(absolute), nil
}

func doctorAssetRootFromExecutable(executable string) string {
	dir := filepath.Dir(executable)
	if filepath.Base(dir) == "go" && filepath.Base(filepath.Dir(dir)) == "bin" {
		dir = filepath.Dir(dir)
	}
	return filepath.Dir(dir)
}

func repairDoctorRules(root string) error {
	assetRoot, err := globalDoctorAssetRoot()
	if err != nil {
		return err
	}
	return repairDoctorRulesFromAssets(root, assetRoot)
}

func repairDoctorRulesFromAssets(root, assetRoot string) error {
	globalRules := filepath.Join(assetRoot, ".agent", "rules")
	projectRules := filepath.Join(root, ".agent", "rules")
	if info, err := os.Lstat(projectRules); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to modify symlinked rules directory: %s", projectRules)
	}
	if info, err := os.Stat(globalRules); err != nil || !info.IsDir() {
		if err != nil {
			return fmt.Errorf("global rules unavailable: %w", err)
		}
		return fmt.Errorf("global rules path is not a directory: %s", globalRules)
	}
	if err := os.MkdirAll(projectRules, 0o755); err != nil {
		return fmt.Errorf("create project rules directory: %w", err)
	}
	entries, err := os.ReadDir(globalRules)
	if err != nil {
		return fmt.Errorf("read global rules directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		source := filepath.Join(globalRules, entry.Name())
		target := filepath.Join(projectRules, entry.Name())
		if _, err := os.Lstat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect project rule %s: %w", entry.Name(), err)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read global rule %s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write project rule %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func findLegacyOrchestratorScripts(root string) []string {
	scriptsDir := filepath.Join(root, "scripts")
	info, err := os.Lstat(scriptsDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return nil
	}
	var legacy []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		full := filepath.Join(scriptsDir, entry.Name())
		fi, statErr := os.Lstat(full)
		if statErr != nil || !fi.Mode().IsRegular() || fi.Size() > 64*1024 {
			continue
		}
		data, readErr := os.ReadFile(full)
		if readErr != nil {
			continue
		}
		if strings.Contains(string(data), "_orchestrator.py") {
			legacy = append(legacy, full)
		}
	}
	sort.Strings(legacy)
	return legacy
}

func configuredWebServiceName(project config.Project) string {
	for _, lang := range []string{"typescript", "javascript", "web"} {
		if svc, ok := project.ServiceForLanguage(lang); ok && svc.Name != "" {
			return svc.Name
		}
	}
	return "webdashboard"
}

func renderMigratedProxyScript(scriptName string, project config.Project) (string, string) {
	switch scriptName {
	case "_npm.sh", "_bun.sh":
		svc := configuredWebServiceName(project)
		body := fmt.Sprintf("#!/usr/bin/env bash\n# Managed by LeeDevKit — hermetic container execution wrapper\nset -euo pipefail\nif [[ \"${1:-}\" == \"bun\" || \"${1:-}\" == \"npm\" || \"${1:-}\" == \"npx\" || \"${1:-}\" == \"bunx\" ]]; then\n    exec leedevkit exec %s -- \"$@\"\nfi\nexec leedevkit exec %s -- bun \"$@\"\n", svc, svc)
		return svc, body
	case "_go.sh":
		svc := configuredServiceName(project, "go")
		body := fmt.Sprintf("#!/usr/bin/env bash\n# Managed by LeeDevKit — hermetic container execution wrapper\nset -euo pipefail\nif [[ \"${1:-}\" == \"go\" || \"${1:-}\" == \"gofmt\" ]]; then\n    exec leedevkit exec %s -- \"$@\"\nfi\nexec leedevkit exec %s -- go \"$@\"\n", svc, svc)
		return svc, body
	case "_cargo.sh":
		svc := configuredServiceName(project, "rust")
		if svc == "rust" {
			if _, ok := project.Services["apiserver"]; ok {
				svc = "apiserver"
			}
		}
		body := fmt.Sprintf("#!/usr/bin/env bash\n# Managed by LeeDevKit — hermetic container execution wrapper\nset -euo pipefail\nif [[ \"${1:-}\" == \"cargo\" || \"${1:-}\" == \"rustc\" ]]; then\n    exec leedevkit exec %s -- \"$@\"\nfi\nexec leedevkit exec %s -- cargo \"$@\"\n", svc, svc)
		return svc, body
	case "_diesel.sh":
		svc := configuredServiceName(project, "rust")
		if svc == "rust" {
			if _, ok := project.Services["apiserver"]; ok {
				svc = "apiserver"
			}
		}
		body := fmt.Sprintf("#!/usr/bin/env bash\n# Managed by LeeDevKit — hermetic container execution wrapper\nset -euo pipefail\nexec leedevkit exec %s -- diesel \"$@\"\n", svc)
		return svc, body
	default:
		svc := ""
		names := make([]string, 0, len(project.Services))
		for name := range project.Services {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) > 0 {
			svc = names[0]
		} else {
			svc = "go"
		}
		body := fmt.Sprintf("#!/usr/bin/env bash\n# Managed by LeeDevKit — hermetic container execution wrapper\nset -euo pipefail\nexec leedevkit exec %s -- \"$@\"\n", svc)
		return svc, body
	}
}

func repairDoctorProjectArtifacts(root string, stderr io.Writer) error {
	claudePath := filepath.Join(root, "CLAUDE.md")
	if info, err := os.Lstat(claudePath); err == nil && info.Mode().IsRegular() {
		if data, readErr := os.ReadFile(claudePath); readErr == nil && runtimepkg.HasClaudeMDIssue(string(data)) {
			if syncErr := runtimepkg.SyncClaudeMD(root); syncErr != nil {
				return syncErr
			}
			if stderr != nil {
				fmt.Fprintf(stderr, "Repaired CLAUDE.md context markers: %s\n", claudePath)
			}
		}
	}
	legacyScripts := findLegacyOrchestratorScripts(root)
	if len(legacyScripts) == 0 {
		return nil
	}
	projectCfg, _ := config.Load(filepath.Join(root, "leedevkit.toml"))
	for _, scriptPath := range legacyScripts {
		svc, updated := renderMigratedProxyScript(filepath.Base(scriptPath), projectCfg)
		tmp, err := os.CreateTemp(filepath.Dir(scriptPath), ".proxy-fix-*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()
		if _, err := io.WriteString(tmp, updated); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return err
		}
		if err := tmp.Chmod(0o755); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpName)
			return err
		}
		if err := os.Rename(tmpName, scriptPath); err != nil {
			_ = os.Remove(tmpName)
			return err
		}
		if stderr != nil {
			fmt.Fprintf(stderr, "Migrated legacy proxy script %s -> leedevkit exec %s\n", scriptPath, svc)
		}
	}
	return nil
}

func warnLegacyProjectArtifacts(root string, stderr io.Writer) {
	if stderr == nil {
		return
	}
	if !runtimepkg.IsDevkitSourceRepo(root) {
		legacyLauncher := filepath.Join(root, "leedevkit")
		if _, err := os.Lstat(legacyLauncher); err == nil {
			fmt.Fprintf(stderr, "warning: legacy project launcher detected and left unchanged (shadows global install if invoked as ./leedevkit): %s\n", legacyLauncher)
		}
	}
	localRuntime := filepath.Join(root, ".leedevkit")
	if _, err := os.Lstat(localRuntime); err == nil {
		fmt.Fprintf(stderr, "warning: legacy local .leedevkit/ directory detected (shadows global install; left unchanged): %s\n", localRuntime)
	}
	localBackup := filepath.Join(root, ".leedevkit.bak")
	if _, err := os.Lstat(localBackup); err == nil {
		fmt.Fprintf(stderr, "warning: legacy .leedevkit.bak/ artifact detected: %s\n", localBackup)
	}
	claudePath := filepath.Join(root, "CLAUDE.md")
	if info, err := os.Lstat(claudePath); err == nil && info.Mode().IsRegular() {
		if data, readErr := os.ReadFile(claudePath); readErr == nil && runtimepkg.HasClaudeMDIssue(string(data)) {
			fmt.Fprintf(stderr, "warning: CLAUDE.md has duplicate or legacy LeeDevKit context markers (run: leedevkit init or leedevkit doctor --fix): %s\n", claudePath)
		}
	}
	for _, scriptPath := range findLegacyOrchestratorScripts(root) {
		fmt.Fprintf(stderr, "warning: legacy proxy script references removed _orchestrator.py: %s (use 'leedevkit exec <service> -- ...' or run 'leedevkit doctor --fix')\n", scriptPath)
	}
}

func sweepDoctorStaleProjects(root string, stderr io.Writer) {
	cli := []string{"docker"}
	if resolved, err := resolveProviderForProject(root, environMap(os.Environ())); err == nil && len(resolved.CLI) > 0 {
		cli = resolved.CLI
	}
	projectCfg, _ := config.Load(filepath.Join(root, "leedevkit.toml"))
	sweepDoctorStaleProjectsWithNamespace(context.Background(), cli, projectCfg.Namespace, process.Run, stderr)
}

func sweepDoctorStaleProjectsWithRunner(ctx context.Context, cli []string, run func(context.Context, process.Request) process.Result, stderr io.Writer) ownership.SweepResult {
	return sweepDoctorStaleProjectsWithNamespace(ctx, cli, "", run, stderr)
}

func sweepDoctorStaleProjectsWithNamespace(ctx context.Context, cli []string, namespace string, run func(context.Context, process.Request) process.Result, stderr io.Writer) ownership.SweepResult {
	result := ownership.SweepRuntime(ctx, ownership.RuntimeSweepOptions{CLI: cli, Namespace: namespace, MaxAge: 0, Run: run})
	if stderr != nil {
		if result.Cleaned > 0 {
			fmt.Fprintf(stderr, "Cleaned %d stale test/infra project(s).\n", result.Cleaned)
		}
		for _, err := range result.Errors {
			fmt.Fprintf(stderr, "warning: stale project sweep: %s\n", process.RedactDiagnostic(err.Error()))
		}
	}
	return result
}

func runDoctor(root string, stdout, stderr io.Writer) int {
	assetRoot, err := globalDoctorAssetRoot()
	if err != nil {
		fmt.Fprintf(stderr, "warning: global DevKit assets unavailable: %v\n", err)
		return runDoctorWithAssets(root, "", stdout, stderr)
	}
	return runDoctorWithAssets(root, assetRoot, stdout, stderr)
}

func runDoctorWithAssets(root, assetRoot string, stdout, stderr io.Writer) int {
	failed := false
	fmt.Fprintln(stderr, "LeeDevKit System Doctor")

	configPath := filepath.Join(root, "leedevkit.toml")
	if _, err := os.Stat(configPath); err != nil {
		fmt.Fprintf(stderr, "warning: leedevkit.toml: %v\n", err)
		failed = true
	} else {
		fmt.Fprintf(stderr, "Project config: %s\n", configPath)
	}

	agentDir := filepath.Join(root, ".agent")
	if info, err := os.Lstat(agentDir); err != nil {
		fmt.Fprintf(stderr, "warning: .agent directory missing (run: leedevkit init): %v\n", err)
		failed = true
	} else if info.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintln(stderr, "warning: .agent is a symlink — expected real directory (run: leedevkit init)")
		failed = true
	} else {
		entries, _ := os.ReadDir(filepath.Join(agentDir, "rules"))
		count := 0
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				count++
			}
		}
		fmt.Fprintf(stderr, ".agent: real directory, %d rulebooks\n", count)
	}

	if assetRoot == "" {
		fmt.Fprintln(stderr, "warning: global DevKit assets unavailable")
		failed = true
	} else {
		version := "?"
		if data, err := os.ReadFile(filepath.Join(assetRoot, "VERSION")); err == nil {
			version = strings.TrimSpace(string(data))
		}
		fmt.Fprintf(stderr, "Global DevKit: %s (v%s)\n", assetRoot, version)
		if info, err := os.Stat(assetRoot); err != nil || !info.IsDir() {
			fmt.Fprintln(stderr, "warning: global DevKit assets unavailable")
			failed = true
		} else if info, err := os.Stat(filepath.Join(assetRoot, ".agent", "rules")); err != nil || !info.IsDir() {
			fmt.Fprintln(stderr, "warning: global DevKit rules unavailable")
			failed = true
		} else {
			fmt.Fprintln(stderr, "Global DevKit assets: available")
		}
	}

	warnLegacyProjectArtifacts(root, stderr)

	for _, name := range []string{"docker", "podman", "podman-compose", "kiri"} {
		path, err := exec.LookPath(name)
		if err != nil {
			fmt.Fprintf(stderr, "runtime %s: unavailable\n", name)
			continue
		}
		result := process.Run(context.Background(), process.Request{Args: []string{path, "--version"}, Timeout: 5 * time.Second})
		if result.ExitCode == 0 {
			fmt.Fprintf(stderr, "runtime %s: installed (%s)\n", name, path)
		} else {
			fmt.Fprintf(stderr, "runtime %s: version check failed\n", name)
		}
	}

	_ = failed
	return 0
}
