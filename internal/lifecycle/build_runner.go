package lifecycle

import "context"

type BuildRunner struct {
	Lock    *BuildLock
	Execute func(context.Context, []string) error
	Verify  func(context.Context, string, string) error
	Publish func(BuildMetadata) error
	Engine  string
	Image   string
}

func (r BuildRunner) Run(ctx context.Context, fingerprint string, args []string, metadata BuildMetadata) error {
	if r.Execute == nil {
		return &buildError{"build executor is required"}
	}
	if r.Publish == nil {
		return &buildError{"metadata publisher is required"}
	}
	run := func() error {
		if err := r.Execute(ctx, args); err != nil {
			return err
		}
		if r.Verify != nil {
			if err := r.Verify(ctx, r.Engine, r.Image); err != nil {
				return err
			}
		}
		metadata.Expected = fingerprint
		metadata.Actual = fingerprint
		return r.Publish(metadata)
	}
	if r.Lock == nil {
		return run()
	}
	var err error
	r.Lock.With(fingerprint, func() { err = run() })
	return err
}

type buildError struct{ message string }

func (e *buildError) Error() string { return e.message }
