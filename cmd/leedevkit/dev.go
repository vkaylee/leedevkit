package main

import (
	"context"
	"fmt"
	"io"

	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

func devComposeArgs(compose []string, operation string) ([]string, error) {
	return devComposeArgsWithOptions(compose, operation, "", nil)
}

func devComposeArgsWithOptions(compose []string, operation, service string, command []string) ([]string, error) {
	if operation != "up" && operation != "down" && operation != "ps" && operation != "logs" && operation != "exec" {
		return nil, fmt.Errorf("unsupported dev operation %q", operation)
	}
	args := append([]string{}, compose...)
	args = append(args, "-p", "leedevkit-dev", "-f", "docker-compose.yml", "-f", ".compose/docker-compose.dev.yml", operation)
	switch operation {
	case "up":
		args = append(args, "-d")
	case "logs":
		args = append(args, "-f")
		if service != "" {
			args = append(args, service)
		}
	case "exec":
		if service == "" {
			return nil, fmt.Errorf("dev exec requires service")
		}
		args = append(args, service)
		if len(command) == 0 {
			args = append(args, "bash")
		} else {
			args = append(args, command...)
		}
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
