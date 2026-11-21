package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

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
	return boolExit(failed)
}

func boolExit(failed bool) int {
	if failed {
		return 1
	}
	return 0
}

func sortedRulebooks(root string) []string {
	entries, _ := os.ReadDir(filepath.Join(root, ".agent", "rules"))
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			result = append(result, entry.Name())
		}
	}
	sort.Strings(result)
	return result
}
