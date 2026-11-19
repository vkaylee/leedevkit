package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/vkaylee/leedevkit/internal/gates"
)

type gateProtocolRequest struct {
	SchemaVersion int          `json:"schema_version"`
	Gates         []gates.Gate `json:"gates"`
}

type gateProtocolResponse struct {
	SchemaVersion int          `json:"schema_version"`
	Gates         []gates.Gate `json:"gates"`
}

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
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--plan-gates" {
		var request gateProtocolRequest
		if err := json.NewDecoder(stdin).Decode(&request); err != nil {
			fmt.Fprintf(stderr, "malformed gate request: %v\n", err)
			return 2
		}
		if request.SchemaVersion != 1 {
			fmt.Fprintf(stderr, "unsupported gate schema version: %d\n", request.SchemaVersion)
			return 2
		}
		response := gateProtocolResponse{SchemaVersion: 1, Gates: request.Gates}
		if err := json.NewEncoder(stdout).Encode(response); err != nil {
			fmt.Fprintf(stderr, "failed to write gate response: %v\n", err)
			return 2
		}
		return 0
	}
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
