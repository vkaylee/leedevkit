package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

func validateInfraProject(root string) error {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.HasPrefix(string(data), "module github.com/vkaylee/leedevkit\n") {
		return fmt.Errorf("infrastructure source checks require the LeeDevKit source repository")
	}
	return nil
}

func validateInfraFlags(args []string) error {
	for _, flag := range args {
		if flag != "--json" && flag != "--quiet" {
			return fmt.Errorf("unsupported flag for infrastructure checks: %s", flag)
		}
	}
	return nil
}

// ponytail: infra remains partial until legacy regression behavior has Go coverage; add parity tests before declaring full regression.
func runInfra(root string, flags testplan.Flags, run func(context.Context, process.Request) process.Result, stream io.Writer) testplan.RunResult {
	checks := [][]string{
		{"docker", "run", "--rm", "-v", root + ":/workspace:ro", "-w", "/workspace", "golang:1.26.11-bookworm", "go", "test", "./..."},
		{"docker", "run", "--rm", "-v", root + ":/workspace:ro", "-w", "/workspace", "golang:1.26.11-bookworm", "sh", "-c", `files=$(gofmt -l $(find cmd internal -name '*.go' -type f)); if [ -n "$files" ]; then printf 'unformatted Go files:\n%s\n' "$files"; exit 1; fi`},
		{"docker", "run", "--rm", "-v", root + ":/workspace:ro", "-w", "/workspace", "golang:1.26.11-bookworm", "go", "vet", "./..."},
		{"docker", "run", "--rm", "-v", root + ":/workspace:ro", "-w", "/workspace", "golang:1.26.11-bookworm", "sh", "-c", `set -eu; profile=$(mktemp); trap 'rm -f "$profile"' EXIT; go test -coverpkg=./... -coverprofile="$profile" ./...; go tool cover -func="$profile" | awk '/^total:/ {found=1; print; if ($3+0 < 80) exit 1} END {if (!found) exit 1}'`},
		{"bash", "-n", "install.sh"},
		{"bash", "-n", "bootstrap.sh"},
		{"bash", "-n", "bin/leedevkit"},
		{"bash", "-n", "leedevkit"},
		{"bash", "-n", "tests/install-test.sh"},
		{"bash", "-n", "scripts/test-skill-utilities.sh"},
		{"bash", "tests/install-test.sh"},
		{"bash", "scripts/test-skill-utilities.sh"},
	}
	flags.Target = "infra"
	result := testplan.Run(flags, testplan.ExecutorFunc(func(phase testplan.Phase) testplan.PhaseResult {
		if phase.Name != "Infrastructure" {
			return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "focused infrastructure checks are not supported"}
		}
		status, code := "passed", 0
		var errors []string
		for _, command := range checks {
			var output strings.Builder
			result := run(context.Background(), process.Request{Args: command, Dir: root, Env: os.Environ(), Stdout: &output, Stderr: &output})
			if result.ExitCode != 0 && output.Len() > 0 {
				result.Stderr = []byte(output.String())
			}
			if result.ExitCode != 0 {
				if code == 0 {
					code = result.ExitCode
				}
				status = "failed"
				errors = append(errors, fmt.Sprintf("%s: %s", strings.Join(command, " "), result.Diagnostic()))
			}
		}
		return testplan.PhaseResult{Status: status, ExitCode: code, Diagnostic: strings.Join(errors, "; "), Command: "source-quality checks"}
	}), stream)
	// Go checks do not yet establish parity with the removed Python regression suite.
	if result.Status == "passed" {
		result.Status = "partial"
		result.FullRegression = false
		result.ExitCode = 2
	}
	return result
}
