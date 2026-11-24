package gates

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/vkaylee/leedevkit/internal/process"
)

type ComposeRunner struct {
	Compose               []string
	Project               string
	Service               string
	Workdir               string
	Dir                   string
	Env                   []string
	Execute               func(context.Context, process.Request) process.Result
	Output                io.Writer
	AllowTruncatedCapture bool
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
	var stdout, stderr boundedCapture
	var out, errOut io.Writer = &stdout, &stderr
	if r.Output != nil {
		out, errOut = io.MultiWriter(r.Output, &stdout), io.MultiWriter(r.Output, &stderr)
	}
	result := run(ctx, process.Request{Args: args, Dir: r.Dir, Env: append([]string(nil), r.Env...), Stdout: out, Stderr: errOut})
	if len(result.Stdout) == 0 {
		result.Stdout = append([]byte(nil), stdout.Bytes()...)
	}
	if len(result.Stderr) == 0 {
		result.Stderr = append([]byte(nil), stderr.Bytes()...)
	}
	if (stdout.overflow || stderr.overflow) && !(r.AllowTruncatedCapture && r.Output != nil) {
		result.ExitCode = 1
		result.Err = fmt.Errorf("compose output exceeds capture limit; see task log")
	}
	return result
}

// ponytail: version and pattern output is capped at 1 MiB per stream; use streaming parsing to lift the limit.
type boundedCapture struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedCapture) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (1 << 20) - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
