package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `leedevkit — Enterprise DevKit for AI-first projects

Usage: leedevkit <command> [args...]

Commands:
  test     Run tests       (leedevkit test infra --lint-only)
  dev      Dev environment (leedevkit dev up|down)
  init     Initialize a project with leedevkit
  update   Update the devkit to the latest (or pinned) release
  doctor   System health check
  skills   Browse and install community skills
  version  Show devkit version

Run 'leedevkit <command> --help' for details.
`

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return 0
	case "test", "dev", "init", "doctor", "skills", "update":
		fmt.Fprintf(stderr, "Go core command not implemented: %s\n", args[0])
		return 2
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\nRun 'leedevkit' without arguments for usage.\n", args[0])
		return 1
	}
}
