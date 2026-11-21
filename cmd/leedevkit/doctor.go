package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

func repairDoctorRules(root string) error {
	runtimeRules := filepath.Join(root, ".leedevkit", ".agent", "rules")
	projectRules := filepath.Join(root, ".agent", "rules")
	if info, err := os.Lstat(projectRules); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to modify symlinked rules directory: %s", projectRules)
	}
	if info, err := os.Stat(runtimeRules); err != nil || !info.IsDir() {
		if err != nil {
			return fmt.Errorf("runtime rules unavailable: %w", err)
		}
		return fmt.Errorf("runtime rules path is not a directory: %s", runtimeRules)
	}
	if err := os.MkdirAll(projectRules, 0o755); err != nil {
		return fmt.Errorf("create project rules directory: %w", err)
	}
	entries, err := os.ReadDir(runtimeRules)
	if err != nil {
		return fmt.Errorf("read runtime rules directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		source := filepath.Join(runtimeRules, entry.Name())
		target := filepath.Join(projectRules, entry.Name())
		if _, err := os.Lstat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect project rule %s: %w", entry.Name(), err)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read runtime rule %s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write project rule %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func runDoctor(root string, stdout, stderr io.Writer) int {
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

	runtime := filepath.Join(root, ".leedevkit")
	if info, err := os.Stat(runtime); err != nil {
		fmt.Fprintf(stderr, "warning: DevKit runtime unavailable: %v\n", err)
		failed = true
	} else {
		version := "?"
		if data, err := os.ReadFile(filepath.Join(runtime, "VERSION")); err == nil {
			version = strings.TrimSpace(string(data))
		}
		fmt.Fprintf(stderr, "DevKit: %s (v%s)\n", runtime, version)
		if info.IsDir() {
			fmt.Fprintln(stderr, "DevKit runtime: available")
		}
	}

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

	for _, port := range []int{3000, 8000, 5432} {
		_ = port
	}
	_ = failed
	return 0
}
