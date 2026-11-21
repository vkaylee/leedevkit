package run

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/vkaylee/leedevkit/internal/ownership"
	"github.com/vkaylee/leedevkit/internal/process"
)

type ExecuteRequest struct {
	Plan        Request
	Owned       bool
	CleanupBase []string
	LogRoot     string
	LogName     string
	Timeout     time.Duration
	Stdout      io.Writer
	Stderr      io.Writer
	RunProcess  func(context.Context, process.Request) process.Result
	RunCompose  func([]string) error
}

type ExecuteResult struct {
	Process process.Result
	Err     error
}

func Execute(ctx context.Context, request ExecuteRequest) (result ExecuteResult) {
	args, err := Plan(request.Plan)
	if err != nil {
		return ExecuteResult{Err: err}
	}
	runProcess := request.RunProcess
	if runProcess == nil {
		runProcess = process.Run
	}
	runCompose := request.RunCompose
	if runCompose == nil {
		return ExecuteResult{Err: fmt.Errorf("compose executor is required")}
	}
	session, err := ownership.Open(request.Plan.Project, request.Owned)
	if err != nil {
		return ExecuteResult{Err: err}
	}
	defer func() {
		cleanupErr := session.Close(func() error {
			cleanup := ownership.Cleanup(request.CleanupBase, runCompose)
			return cleanup.Err
		})
		if result.Err == nil && cleanupErr != nil {
			result.Err = cleanupErr
		}
	}()
	var log io.WriteCloser
	if request.LogRoot != "" && request.LogName != "" {
		log, err = ownership.OpenLog(request.LogRoot, request.Plan.Project, request.LogName)
		if err != nil {
			return ExecuteResult{Err: err}
		}
		defer log.Close()
	}
	stdout, stderr := request.Stdout, request.Stderr
	if log != nil {
		stdout, stderr = log, log
	}
	processRequest := process.Request{Args: args, Stdout: stdout, Stderr: stderr, Timeout: request.Timeout}
	result.Process = runProcess(ctx, processRequest)
	if result.Process.Err != nil {
		result.Err = result.Process.Err
	}
	return result
}
