package main

import (
	"context"
	"fmt"
	"io"

	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

func devComposeArgs(compose []string, operation string) ([]string, error) {
	if operation != "up" && operation != "down" {
		return nil, fmt.Errorf("unsupported dev operation %q", operation)
	}
	args := append([]string{}, compose...)
	args = append(args,
		"-p", "leedevkit-dev",
		"-f", "docker-compose.yml",
		"-f", ".compose/docker-compose.dev.yml",
		operation,
	)
	if operation == "up" {
		args = append(args, "-d")
	}
	return args, nil
}

func runDev(operation string, config provider.Provider, root string, stdout, stderr io.Writer, run func(context.Context, process.Request) process.Result) int {
	args, err := devComposeArgs(config.Compose, operation)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if run == nil {
		run = process.Run
	}
	result := run(context.Background(), process.Request{Args: args, Dir: root, Stdout: stdout, Stderr: stderr})
	if result.ExitCode != 0 && result.Err != nil {
		fmt.Fprintln(stderr, result.Err)
	}
	return result.ExitCode
}
