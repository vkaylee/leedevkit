package lifecycle

import (
	"context"
	"errors"
	"fmt"
)

type Executor struct {
	UpArgs    []string
	DownArgs  []string
	Execute   func(context.Context, []string) error
	Readiness func(context.Context) error
}

func (e Executor) Run(ctx context.Context, work func() error) (result error) {
	if e.Execute == nil {
		return errors.New("lifecycle process runner is required")
	}
	if err := e.Execute(ctx, e.UpArgs); err != nil {
		_ = e.Execute(ctx, e.DownArgs)
		return fmt.Errorf("lifecycle startup: %w", err)
	}
	if e.Readiness != nil {
		if err := e.Readiness(ctx); err != nil {
			_ = e.Execute(ctx, e.DownArgs)
			return fmt.Errorf("lifecycle readiness: %w", err)
		}
	}
	defer func() {
		if err := e.Execute(ctx, e.DownArgs); result == nil && err != nil {
			result = fmt.Errorf("lifecycle teardown: %w", err)
		}
	}()
	if work == nil {
		return errors.New("lifecycle work is required")
	}
	return work()
}
