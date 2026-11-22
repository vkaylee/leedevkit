package lifecycle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/vkaylee/leedevkit/internal/process"
)

type ComposeExecutor struct {
	Run    func(context.Context, process.Request) process.Result
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type CommandError struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Err      error
}

func (e *CommandError) Error() string {
	parts := []string{fmt.Sprintf("compose exited with code %d", e.ExitCode)}
	if text := strings.TrimSpace(process.RedactDiagnostic(string(e.Stderr))); text != "" {
		parts = append(parts, "stderr="+text)
	}
	if text := strings.TrimSpace(process.RedactDiagnostic(string(e.Stdout))); text != "" {
		parts = append(parts, "stdout="+text)
	}
	if e.Err != nil && len(e.Stderr) == 0 && len(e.Stdout) == 0 {
		parts = append(parts, process.RedactDiagnostic(e.Err.Error()))
	}
	return strings.Join(parts, ": ")
}

func (e *CommandError) Unwrap() error { return e.Err }

func (e ComposeExecutor) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("compose argv is required")
	}
	run := e.Run
	if run == nil {
		run = process.Run
	}
	var stdout, stderr bytes.Buffer
	stdoutWriter := io.Writer(&stdout)
	if e.Stdout != nil {
		stdoutWriter = io.MultiWriter(&stdout, e.Stdout)
	}
	stderrWriter := io.Writer(&stderr)
	if e.Stderr != nil {
		stderrWriter = io.MultiWriter(&stderr, e.Stderr)
	}
	result := run(ctx, process.Request{Args: args, Dir: e.Dir, Env: append([]string(nil), e.Env...), Stdout: stdoutWriter, Stderr: stderrWriter})
	if len(result.Stdout) == 0 {
		result.Stdout = stdout.Bytes()
	}
	if len(result.Stderr) == 0 {
		result.Stderr = stderr.Bytes()
	}
	if result.ExitCode != 0 {
		return &CommandError{ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr, Err: result.Err}
	}
	return nil
}
