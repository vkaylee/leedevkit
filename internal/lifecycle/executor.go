package lifecycle

import (
	"context"
	"errors"
)

type Executor struct {
	UpArgs        []string
	DownArgs      []string
	ReadinessArgs []string
	Execute       func(context.Context, []string) error
	Readiness     func(context.Context) error
	Report        func(string, []string, error)
}

type Failure struct {
	Phase string
	Args  []string
	Err   error
}

func (f *Failure) Error() string { return f.Phase + ": " + f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

func (e Executor) Run(ctx context.Context, work func() error) (result error) {
	if e.Execute == nil {
		return errors.New("lifecycle process runner is required")
	}
	if err := e.Execute(ctx, e.UpArgs); err != nil {
		e.report("startup", e.UpArgs, err)
		_ = e.Execute(context.WithoutCancel(ctx), e.DownArgs)
		return &Failure{Phase: "startup", Args: append([]string(nil), e.UpArgs...), Err: err}
	}
	if e.Readiness != nil {
		if err := e.Readiness(ctx); err != nil {
			e.report("readiness", e.ReadinessArgs, err)
			_ = e.Execute(context.WithoutCancel(ctx), e.DownArgs)
			return &Failure{Phase: "readiness", Args: append([]string(nil), e.ReadinessArgs...), Err: err}
		}
	}
	defer func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if err := e.Execute(cleanupCtx, e.DownArgs); result == nil && err != nil {
			e.report("teardown", e.DownArgs, err)
			result = &Failure{Phase: "teardown", Args: append([]string(nil), e.DownArgs...), Err: err}
		}
	}()
	if work == nil {
		return errors.New("lifecycle work is required")
	}
	return work()
}

func (e Executor) report(phase string, args []string, err error) {
	if e.Report != nil {
		e.Report(phase, append([]string(nil), args...), err)
	}
}
