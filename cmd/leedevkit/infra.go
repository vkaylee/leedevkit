package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/ownership"
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
	var lintOnly, unitOnly bool
	for _, flag := range args {
		switch flag {
		case "--json", "--quiet", "--json-stream", "--failed-only", "--compact", "--build":
		case "--lint-only":
			lintOnly = true
		case "--unit-only":
			unitOnly = true
		default:
			return fmt.Errorf("unsupported flag for infrastructure checks: %s", flag)
		}
	}
	if lintOnly && unitOnly {
		return fmt.Errorf("infra cannot combine --lint-only and --unit-only")
	}
	return nil
}

type infraCheck struct {
	name       string
	category   string
	args       []string
	compactLog bool
}

type infraWriteError struct{ err error }

func (e *infraWriteError) Error() string { return e.err.Error() }
func (e *infraWriteError) Unwrap() error { return e.err }

func runInfra(root string, flags testplan.Flags, run func(context.Context, process.Request) process.Result, stream io.Writer, quietMode ...bool) testplan.RunResult {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runInfraContext(ctx, root, flags, run, stream, quietMode...)
}

func runInfraContext(ctx context.Context, root string, flags testplan.Flags, run func(context.Context, process.Request) process.Result, stream io.Writer, quietMode ...bool) testplan.RunResult {
	compact := flags.FailedOnly || flags.Compact
	quiet := compact || flags.JSONStream || (len(quietMode) > 0 && quietMode[0])
	if compact {
		flags.JSONStream = false
	}
	project := uniqueInfraProject(root)
	if session, err := ownership.Open(project, true); err == nil {
		defer session.Close(nil)
	}
	composeFile := resolveInfraComposeFile(root)
	compose := []string{"docker", "compose"}
	if resolved, err := resolveProviderForProject(root, environMap(os.Environ())); err == nil && len(resolved.Compose) > 0 {
		compose = resolved.Compose
	}
	upArgs := infraUpCommandWith(compose, project, composeFile, flags.Build)
	downArgs := infraDownCommandWith(compose, project, composeFile)
	checks := infraChecksForFlags(flags, quiet)
	flags.Target = "infra"
	var failureLogs []string
	failureDetails := make(map[string]testplan.FailureLogDetail)
	result := testplan.RunContext(ctx, flags, testplan.ContextExecutorFunc(func(phaseCtx context.Context, phase testplan.Phase) testplan.PhaseResult {
		if phase.Name != "Infrastructure" && phase.Name != "Linting" && phase.Name != "Unit Tests" {
			return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "focused infrastructure checks are not supported"}
		}
		logDir, err := createInfraLogDir(root)
		if err != nil {
			return testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: "infrastructure task logging unavailable: " + process.RedactDiagnostic(err.Error())}
		}
		env := withProjectRoot(os.Environ(), root)
		lifecycleLogName := "infra-lifecycle.log"
		lifecycleRelLog := filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), lifecycleLogName))
		executor := lifecycle.Executor{
			UpArgs:   upArgs,
			DownArgs: downArgs,
			Execute: func(ctx context.Context, args []string) error {
				output, _, err := openPhaseOutput(logDir, lifecycleLogName, "[infra] ", quiet, stream, true)
				if err != nil {
					return err
				}
				child := run(ctx, process.Request{Args: args, Dir: root, Env: env, Stdout: output, Stderr: output, Timeout: infraChildTimeout})
				if err := output.Close(); err != nil {
					return &infraWriteError{err: err}
				}
				if child.ExitCode != 0 {
					return errors.New(child.Diagnostic())
				}
				return nil
			},
		}
		var outcome testplan.PhaseResult
		lifecycleErr := executor.Run(phaseCtx, func() error {
			status, code := "passed", 0
			var failures []string
			var firstLog string
			var goUnitFailed, goBuildFailed bool
			for index, check := range checks {
				if check.category == "coverage" && goUnitFailed {
					continue
				}
				if check.name == "go vet ./..." && goBuildFailed {
					continue
				}
				if err := phaseCtx.Err(); err != nil {
					if status == "passed" {
						outcome = testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "infrastructure checks canceled: " + process.RedactDiagnostic(err.Error()), Command: "source-quality checks", Log: firstLog}
						return err
					}
					break
				}
				command := infraComposeCommandWith(compose, root, project, composeFile, check.args)
				logName := infraLogName(index + 1)
				relLog := filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))
				output, _, err := openPhaseOutput(logDir, logName, fmt.Sprintf("[infra-%02d] ", index+1), quiet, stream, false)
				if err != nil {
					outcome = testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: fmt.Sprintf("infrastructure logging failed before task %02d (log: %s): %s", index+1, relLog, process.RedactDiagnostic(err.Error()))}
					return errors.New(outcome.Diagnostic)
				}
				child := run(phaseCtx, process.Request{Args: command, Dir: root, Env: env, Stdout: output, Stderr: output, Timeout: infraChildTimeout})
				if err := output.Close(); err != nil {
					outcome = testplan.PhaseResult{Status: "failed", ExitCode: 1, Diagnostic: fmt.Sprintf("infrastructure output write failed (log: %s): %s", relLog, process.RedactDiagnostic(err.Error()))}
					return errors.New(outcome.Diagnostic)
				}
				if child.ExitCode != 0 {
					failureLogs = append(failureLogs, relLog)
					if code == 0 {
						code = child.ExitCode
						firstLog = relLog
					}
					status = "failed"
					checkDiag := fmt.Sprintf("%s: %s (log: %s)", check.name, child.Diagnostic(), relLog)
					failures = append(failures, checkDiag)
					failureDetails[relLog] = testplan.FailureLogDetail{
						Label:          check.name,
						Diagnostic:     checkDiag,
						ExtractExcerpt: check.compactLog,
					}
					if check.category == "unit" {
						goUnitFailed = true
						if data, readErr := readLimitedFile(filepath.Join(logDir, logName), 1<<20); readErr == nil && hasGoBuildFailure(data) {
							goBuildFailed = true
						}
					}
				}
				if phaseCtx.Err() != nil {
					break
				}
			}
			outcome = testplan.PhaseResult{Status: status, ExitCode: code, Diagnostic: strings.Join(failures, "; "), Command: "source-quality checks", Log: firstLog}
			if status != "passed" {
				return errors.New(outcome.Diagnostic)
			}
			return nil
		})
		if lifecycleErr != nil && (outcome.Status == "" || outcome.Status == "passed") {
			var writeErr *infraWriteError
			if errors.As(lifecycleErr, &writeErr) {
				return testplan.PhaseResult{Status: "failed", ExitCode: 1, Diagnostic: fmt.Sprintf("infrastructure output write failed (log: %s): %s", lifecycleRelLog, process.RedactDiagnostic(writeErr.Error())), Log: lifecycleRelLog}
			}
			status, code := lifecycleFailureStatus(lifecycleErr)
			diagnostic := process.RedactDiagnostic(lifecycleErr.Error())
			commandArgs := upArgs
			var failure *lifecycle.Failure
			if errors.As(lifecycleErr, &failure) && len(failure.Args) > 0 {
				commandArgs = failure.Args
			}
			failureLogs = append(failureLogs, lifecycleRelLog)
			failureDetails[lifecycleRelLog] = testplan.FailureLogDetail{
				Label:          "infra lifecycle",
				Diagnostic:     diagnostic,
				ExtractExcerpt: true,
			}
			return testplan.PhaseResult{Status: status, ExitCode: code, Diagnostic: diagnostic, LifecycleError: diagnostic, Command: strings.Join(process.RedactArgs(commandArgs), " "), Log: lifecycleRelLog}
		}
		return outcome
	}), stream, func(result *testplan.RunResult) {
		result.FailureLogs = failureLogs
		result.FailureDetails = failureDetails
		var failureOutput io.Writer
		if compact {
			failureOutput = stream
		}
		collectFailureDetails(root, result, failureOutput)
	})
	return result
}

func resolveInfraComposeFile(root string) string {
	composeFile := filepath.Join(root, "container", "go", "docker-compose.test.yml")
	if !fileExists(composeFile) {
		if assetRoot, err := runtimeAssetRoot(); err == nil {
			composeFile = filepath.Join(assetRoot, "container", "go", "docker-compose.test.yml")
		}
	}
	return composeFile
}

func readLimitedFile(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return data, err
}

func hasGoBuildFailure(data []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var event struct {
				Action      string `json:"Action"`
				FailedBuild string `json:"FailedBuild"`
			}
			if json.Unmarshal([]byte(line), &event) == nil {
				if event.Action == "build-fail" || event.FailedBuild != "" {
					return true
				}
			}
			continue
		}
		if strings.HasPrefix(line, "FAIL\t") && (strings.HasSuffix(line, "[build failed]") || strings.HasSuffix(line, "[setup failed]")) {
			return true
		}
	}
	return false
}

func uniqueInfraProject(root string) string {
	return fmt.Sprintf("leedevkit-infra-%x-%x-%x", fnv32a(root), os.Getpid(), nextProjectNonce())
}

func infraUpCommand(project, composeFile string, build bool) []string {
	return infraUpCommandWith([]string{"docker", "compose"}, project, composeFile, build)
}

func infraUpCommandWith(compose []string, project, composeFile string, build bool) []string {
	args := append(composeBaseArgs(compose, project, []string{composeFile}), "--profile", "infra", "up", "-d")
	if build {
		args = append(args, "--build")
	}
	return append(args, "--wait", "--wait-timeout", "120", "infra")
}

func infraDownCommand(project, composeFile string) []string {
	return infraDownCommandWith([]string{"docker", "compose"}, project, composeFile)
}

func infraDownCommandWith(compose []string, project, composeFile string) []string {
	return append(composeBaseArgs(compose, project, []string{composeFile}), "--profile", "infra", "--profile", "*", "down", "--remove-orphans", "--timeout", "1")
}

func infraChecksForFlags(flags testplan.Flags, structuredGo bool) []infraCheck {
	goTestArgs := []string{"go", "test", "./..."}
	if structuredGo {
		goTestArgs = []string{"go", "test", "-json", "./..."}
	}
	all := []infraCheck{
		{name: "go test ./...", category: "unit", args: goTestArgs, compactLog: true},
		{name: "gofmt", category: "lint", args: []string{"sh", "-c", `files=$(gofmt -l $(find cmd internal -name '*.go' -type f)); if [ -n "$files" ]; then printf 'unformatted Go files:\n%s\n' "$files"; exit 1; fi`}, compactLog: true},
		{name: "go vet ./...", category: "lint", args: []string{"go", "vet", "./..."}, compactLog: true},
		{name: "go coverage", category: "coverage", args: []string{"sh", "-c", `set -eu; profile=$(mktemp); trap 'rm -f "$profile"' EXIT; go test -coverpkg=./... -coverprofile="$profile" ./...; go tool cover -func="$profile" | awk '/^total:/ {found=1; print; if ($3+0 < 80) exit 1} END {if (!found) exit 1}'`}, compactLog: true},
		{name: "bash -n install.sh", category: "lint", args: []string{"bash", "-n", "install.sh"}, compactLog: true},
		{name: "bash -n bootstrap.sh", category: "lint", args: []string{"bash", "-n", "bootstrap.sh"}, compactLog: true},
		{name: "bash -n bin/leedevkit", category: "lint", args: []string{"bash", "-n", "bin/leedevkit"}, compactLog: true},
		{name: "bash -n leedevkit", category: "lint", args: []string{"bash", "-n", "leedevkit"}, compactLog: true},
		{name: "bash -n tests/install-test.sh", category: "lint", args: []string{"bash", "-n", "tests/install-test.sh"}, compactLog: true},
		{name: "bash -n scripts/test-skill-utilities.sh", category: "lint", args: []string{"bash", "-n", "scripts/test-skill-utilities.sh"}, compactLog: true},
		{name: "bash tests/install-test.sh", category: "script", args: []string{"bash", "tests/install-test.sh"}, compactLog: true},
		{name: "bash scripts/test-skill-utilities.sh", category: "script", args: []string{"bash", "scripts/test-skill-utilities.sh"}, compactLog: true},
	}
	if !flags.LintOnly && !flags.UnitOnly {
		return all
	}
	wantCategory := "lint"
	if flags.UnitOnly {
		wantCategory = "unit"
	}
	selected := make([]infraCheck, 0, len(all))
	for _, check := range all {
		if check.category == wantCategory {
			selected = append(selected, check)
		}
	}
	return selected
}

func runInfraCommands(root, project, composeFile string) [][]string {
	return runInfraCommandsWith([]string{"docker", "compose"}, root, project, composeFile)
}

func runInfraCommandsWith(compose []string, root, project, composeFile string) [][]string {
	checks := infraChecksForFlags(testplan.Flags{}, false)
	commands := make([][]string, len(checks))
	for index, check := range checks {
		commands[index] = infraComposeCommandWith(compose, root, project, composeFile, check.args)
	}
	return commands
}

func infraComposeCommand(root, project, composeFile string, command []string) []string {
	return infraComposeCommandWith([]string{"docker", "compose"}, root, project, composeFile, command)
}

func infraComposeCommandWith(compose []string, root, project, composeFile string, command []string) []string {
	args := append(composeBaseArgs(compose, project, []string{composeFile}), "exec", "-T", "-w", "/workspace", "infra")
	return append(args, command...)
}

const (
	infraChildTimeout = 30 * time.Minute
	maxInfraLogDirs   = 20
)

func infraLogName(index int) string { return fmt.Sprintf("infra-%02d.log", index) }

func createInfraLogDir(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("project root is not a safe directory")
	}
	logsRoot := filepath.Join(root, ".test_logs")
	if err := os.Mkdir(logsRoot, 0o700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("create log root: %w", err)
	}
	info, err = os.Lstat(logsRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("log root is not a safe directory")
	}
	path, err := os.MkdirTemp(logsRoot, "infra-")
	if err != nil {
		return "", fmt.Errorf("create run log directory: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return "", fmt.Errorf("secure run log directory: %w", err)
	}
	pruneInfraLogDirs(logsRoot, filepath.Base(path), maxInfraLogDirs)
	return path, nil
}

func pruneInfraLogDirs(logsRoot, keepName string, maxKeep int) {
	if maxKeep <= 0 {
		return
	}
	entries, err := os.ReadDir(logsRoot)
	if err != nil {
		return
	}
	type dirEntry struct {
		name    string
		path    string
		modTime time.Time
	}
	var candidates []dirEntry
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "infra-") || name == keepName {
			continue
		}
		fullPath := filepath.Join(logsRoot, name)
		info, statErr := os.Lstat(fullPath)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		candidates = append(candidates, dirEntry{name: name, path: fullPath, modTime: info.ModTime()})
	}
	keepOthers := maxKeep - 1
	if len(candidates) <= keepOthers {
		return
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].modTime.After(candidates[j].modTime)
		}
		return candidates[i].name > candidates[j].name
	})
	// ponytail: relies on directory ModTime and fixed retention count; add lockfile liveness checks if >20 concurrent runs are needed.
	for _, old := range candidates[keepOthers:] {
		_ = os.RemoveAll(old.path)
	}
}

type infraPrefixWriter struct {
	writer io.Writer
	prefix string
	start  bool
}

func (w *infraPrefixWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if w.writer == nil {
		return len(data), nil
	}
	written := 0
	for len(data) > 0 {
		if !w.start {
			if _, err := io.WriteString(w.writer, w.prefix); err != nil {
				return written, err
			}
			w.start = true
		}
		index := -1
		for i, b := range data {
			if b == '\n' {
				index = i
				break
			}
		}
		chunk := len(data)
		if index >= 0 {
			chunk = index + 1
		}
		n, err := w.writer.Write(data[:chunk])
		written += n
		data = data[n:]
		if err != nil {
			return written, err
		}
		if n != chunk {
			return written, io.ErrShortWrite
		}
		if index >= 0 {
			w.start = false
		}
	}
	return written, nil
}

type infraOutputWriter struct {
	log  *os.File
	live io.Writer
	mu   sync.Mutex
	err  error
}

func (w *infraOutputWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.log.Write(data)
	if err != nil {
		w.err = err
		return n, err
	}
	if n != len(data) {
		w.err = io.ErrShortWrite
		return n, w.err
	}
	if w.live != nil {
		liveN, liveErr := w.live.Write(data)
		if liveErr != nil {
			w.err = liveErr
			return liveN, liveErr
		}
		if liveN != len(data) {
			w.err = io.ErrShortWrite
			return liveN, w.err
		}
	}
	return len(data), nil
}

func (w *infraOutputWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.log.Close()
	if w.err != nil {
		return w.err
	}
	return err
}
