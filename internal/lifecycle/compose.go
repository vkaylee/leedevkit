package lifecycle

import (
	"context"
	"fmt"

	"github.com/vkaylee/leedevkit/internal/process"
)

type ComposeExecutor struct {
	Run func(context.Context, process.Request) process.Result
}

func (e ComposeExecutor) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("compose argv is required")
	}
	run := e.Run
	if run == nil {
		run = process.Run
	}
	result := run(ctx, process.Request{Args: args})
	if result.ExitCode != 0 {
		if result.Err != nil {
			return result.Err
		}
		return fmt.Errorf("compose exited with code %d", result.ExitCode)
	}
	return nil
}
