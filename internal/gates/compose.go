package gates

import (
	"context"

	"github.com/vkaylee/leedevkit/internal/process"
)

type ComposeRunner struct {
	Compose []string
	Project string
	Service string
	Workdir string
	Execute func(context.Context, process.Request) process.Result
}

func (r ComposeRunner) WithService(name, workdir string) ComposeRunner {
	if name != "" {
		r.Service = name
	}
	if workdir != "" {
		r.Workdir = workdir
	}
	return r
}

func (r ComposeRunner) Run(ctx context.Context, command []string, environment []string) process.Result {
	args := append([]string{}, r.Compose...)
	args = append(args, "-p", r.Project, "exec", "-T")
	if r.Workdir != "" {
		args = append(args, "-w", r.Workdir)
	}
	for _, name := range environment {
		args = append(args, "-e", name)
	}
	args = append(args, r.Service)
	args = append(args, command...)
	run := r.Execute
	if run == nil {
		run = process.Run
	}
	return run(ctx, process.Request{Args: args})
}
