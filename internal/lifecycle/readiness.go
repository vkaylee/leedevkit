package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

type Readiness struct {
	Run func(context.Context, process.Request) process.Result
}

func (r Readiness) Wait(ctx context.Context, base []string, services []string, timeout time.Duration) error {
	if len(base) == 0 || len(services) == 0 {
		return fmt.Errorf("readiness command and services are required")
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	run := r.Run
	if run == nil {
		run = process.Run
	}
	for {
		result := run(ctx, process.Request{Args: append(append([]string(nil), base...), services...)})
		if result.ExitCode == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("readiness timeout for %s", strings.Join(services, ","))
		case <-ticker.C:
		}
	}
}
