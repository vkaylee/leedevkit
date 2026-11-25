package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vkaylee/leedevkit/internal/lifecycle"
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
		if flag != "--json" && flag != "--quiet" && flag != "--json-stream" && flag != "--failed-only" && flag != "--compact" && flag != "--build" {
			return fmt.Errorf("unsupported flag for infrastructure checks: %s", flag)
		}
	}
	return nil
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
	project := fmt.Sprintf("leedevkit-infra-%x-%x", fnv32a(root), os.Getpid())
	composeFile := filepath.Join(root, "container", "go", "docker-compose.test.yml")
	if !fileExists(composeFile) {
		if assetRoot, err := runtimeAssetRoot(); err == nil {
			composeFile = filepath.Join(assetRoot, "container", "go", "docker-compose.test.yml")
		}
	}
	upArgs := infraUpCommand(project, composeFile, flags.Build)
	downArgs := infraDownCommand(project, composeFile)
	checks := runInfraCommands(root, project, composeFile)
	flags.Target = "infra"
	var failureLogs []string
	result := testplan.Run(flags, testplan.ExecutorFunc(func(phase testplan.Phase) testplan.PhaseResult {
		if phase.Name != "Infrastructure" {
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
				logFile, err := os.OpenFile(filepath.Join(logDir, lifecycleLogName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
				if err != nil {
					return err
				}
				var live io.Writer = io.Discard
				if !quiet && stream != nil {
					live = &infraPrefixWriter{writer: stream, prefix: "[infra] "}
				}
				output := &infraOutputWriter{log: logFile, live: live}
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
		lifecycleErr := executor.Run(ctx, func() error {
			status, code := "passed", 0
			var failures []string
			var firstLog string
			for index, command := range checks {
				if err := ctx.Err(); err != nil {
					if status == "passed" {
						outcome = testplan.PhaseResult{Status: "blocked", ExitCode: 1, Diagnostic: "infrastructure checks canceled: " + process.RedactDiagnostic(err.Error()), Command: "source-quality checks", Log: firstLog}
						return err
					}
					break
				}
				logName := infraLogName(index + 1)
				relLog := filepath.ToSlash(filepath.Join(".test_logs", filepath.Base(logDir), logName))
				logFile, err := os.OpenFile(filepath.Join(logDir, logName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
				if err != nil {
					outcome = testplan.PhaseResult{Status: "blocked", ExitCode: 2, Diagnostic: fmt.Sprintf("infrastructure logging failed before task %02d (log: %s): %s", index+1, relLog, process.RedactDiagnostic(err.Error()))}
					return errors.New(outcome.Diagnostic)
				}
				var live io.Writer = io.Discard
				if !quiet && stream != nil {
					live = &infraPrefixWriter{writer: stream, prefix: fmt.Sprintf("[infra-%02d] ", index+1)}
				}
				output := &infraOutputWriter{log: logFile, live: live}
				child := run(ctx, process.Request{Args: command, Dir: root, Env: env, Stdout: output, Stderr: output, Timeout: infraChildTimeout})
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
					failures = append(failures, fmt.Sprintf("%s: %s (log: %s)", process.RedactDiagnostic(strings.Join(command, " ")), child.Diagnostic(), relLog))
				}
				if ctx.Err() != nil {
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
			return testplan.PhaseResult{Status: status, ExitCode: code, Diagnostic: diagnostic, LifecycleError: diagnostic, Command: strings.Join(process.RedactArgs(commandArgs), " "), Log: lifecycleRelLog}
		}
		return outcome
	}), stream, func(result *testplan.RunResult) {
		result.FailureLogs = failureLogs
		var failureOutput io.Writer
		if compact {
			failureOutput = stream
		}
		collectFailureDetails(root, result, failureOutput)
	})
	return result
}

func infraUpCommand(project, composeFile string, build bool) []string {
	args := []string{"docker", "compose", "-p", project, "-f", composeFile, "--profile", "infra", "up", "-d"}
	if build {
		args = append(args, "--build")
	}
	return append(args, "--wait", "--wait-timeout", "120", "infra")
}

func infraDownCommand(project, composeFile string) []string {
	return []string{"docker", "compose", "-p", project, "-f", composeFile, "down", "--remove-orphans", "--timeout", "1"}
}

func runInfraCommands(root, project, composeFile string) [][]string {
	checks := [][]string{
		{"go", "test", "./..."},
		{"sh", "-c", `files=$(gofmt -l $(find cmd internal -name '*.go' -type f)); if [ -n "$files" ]; then printf 'unformatted Go files:\n%s\n' "$files"; exit 1; fi`},
		{"go", "vet", "./..."},
		{"sh", "-c", `set -eu; profile=$(mktemp); trap 'rm -f "$profile"' EXIT; go test -coverpkg=./... -coverprofile="$profile" ./...; go tool cover -func="$profile" | awk '/^total:/ {found=1; print; if ($3+0 < 80) exit 1} END {if (!found) exit 1}'`},
		{"bash", "-n", "install.sh"},
		{"bash", "-n", "bootstrap.sh"},
		{"bash", "-n", "bin/leedevkit"},
		{"bash", "-n", "leedevkit"},
		{"bash", "-n", "tests/install-test.sh"},
		{"bash", "-n", "scripts/test-skill-utilities.sh"},
		{"bash", "tests/install-test.sh"},
		{"bash", "scripts/test-skill-utilities.sh"},
	}
	commands := make([][]string, len(checks))
	for index, check := range checks {
		commands[index] = infraComposeCommand(root, project, composeFile, check)
	}
	return commands
}

func infraComposeCommand(root, project, composeFile string, command []string) []string {
	args := []string{"docker", "compose", "-p", project, "-f", composeFile, "exec", "-T", "-w", "/workspace", "infra"}
	return append(args, command...)
}

const infraChildTimeout = 30 * time.Minute

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
	return path, nil
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
