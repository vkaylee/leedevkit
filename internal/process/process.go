package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type Request struct {
	Args    []string
	Dir     string
	Env     []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Timeout time.Duration
}

type Result struct {
	ExitCode int
	Err      error
	Stdout   []byte
	Stderr   []byte
}

// Run executes argv without a shell and maps managed failures to stable CLI codes.
func Run(ctx context.Context, request Request) Result {
	if len(request.Args) == 0 {
		return Result{ExitCode: 127, Err: errors.New("executable is required")}
	}
	if request.Timeout <= 0 {
		request.Timeout = 30 * time.Minute
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()

	command := exec.Command(request.Args[0], request.Args[1:]...)
	command.Dir = request.Dir
	command.Env = append([]string(nil), request.Env...)
	command.Stdin = request.Stdin
	command.Stdout = request.Stdout
	command.Stderr = request.Stderr
	command.SysProcAttr = processGroupAttributes()
	if err := command.Start(); err != nil {
		return Result{ExitCode: 127, Err: fmt.Errorf("start executable: %w", err)}
	}

	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	select {
	case err := <-wait:
		return resultFromError(err)
	case <-ctx.Done():
		_ = terminateProcessGroup(command.Process)
		<-wait
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{ExitCode: 124, Err: ctx.Err()}
		}
		return Result{ExitCode: 130, Err: ctx.Err()}
	}
}

func resultFromError(err error) Result {
	if err == nil {
		return Result{ExitCode: 0}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code, ok := resultStatus(exitErr); ok {
			return Result{ExitCode: code, Err: err}
		}
	}
	return Result{ExitCode: 1, Err: err}
}

func terminateProcessGroup(process *os.Process) error {
	if process == nil {
		return nil
	}
	return terminateProcess(process)
}
