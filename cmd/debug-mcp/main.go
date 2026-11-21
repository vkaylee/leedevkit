package main

import (
	"fmt"
	"os"

	"github.com/vkaylee/leedevkit/internal/debug"
)

type options struct {
	Environment string
	Manifest    string
	ProjectRoot string
}

func parseOptions(args []string) (options, error) {
	var result options
	for index := 0; index < len(args); index++ {
		if index+1 >= len(args) {
			return options{}, fmt.Errorf("%s requires a value", args[index])
		}
		switch args[index] {
		case "--environment":
			result.Environment = args[index+1]
		case "--manifest":
			result.Manifest = args[index+1]
		case "--project-root":
			result.ProjectRoot = args[index+1]
		default:
			return options{}, fmt.Errorf("unsupported argument %q", args[index])
		}
		index++
	}
	if result.Environment == "" || result.Manifest == "" {
		return options{}, fmt.Errorf("--environment and --manifest are required")
	}
	return result, nil
}

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	root := options.ProjectRoot
	if root == "" {
		root, _ = os.Getwd()
	}
	environment, err := debug.Load(options.Manifest, options.Environment, debug.Options{ProjectRoot: root})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := debug.NewServer(environment).Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
