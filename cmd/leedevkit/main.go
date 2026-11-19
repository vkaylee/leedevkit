package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

func runtimeRoot(executable string) string {
	candidate := filepath.Dir(executable)
	for range 4 {
		if _, err := os.Stat(filepath.Join(candidate, "scripts", "_orchestrator.py")); err == nil {
			return candidate
		}
		candidate = filepath.Dir(candidate)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidate = filepath.Join(cwd, ".leedevkit")
		if _, err := os.Stat(filepath.Join(candidate, "scripts", "_orchestrator.py")); err == nil {
			return candidate
		}
	}
	return ""
}

func runPythonCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "Go core cannot locate executable: %v\n", err)
		return 2
	}
	runtime := runtimeRoot(executable)
	if runtime == "" {
		fmt.Fprintln(stderr, "Python compatibility runtime missing")
		return 2
	}
	orchestrator := filepath.Join(runtime, "scripts", "_orchestrator.py")
	ensureVenv := filepath.Join(runtime, "scripts", "_ensure-venv.sh")
	if _, err := os.Stat(orchestrator); err != nil {
		fmt.Fprintf(stderr, "Python compatibility runtime missing: %s\n", orchestrator)
		return 2
	}
	if args[0] == "doctor" {
		python, err := exec.LookPath("python3")
		if err != nil {
			fmt.Fprintf(stderr, "Python interpreter missing: %v\n", err)
			return 2
		}
		command := exec.Command(python, append([]string{filepath.Join(runtime, "scripts", "_doctor.py")}, args[1:]...)...)
		command.Stdin = stdin
		command.Stdout = stdout
		command.Stderr = stderr
		command.Env = os.Environ()
		if err := command.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode()
			}
			return 2
		}
		return 0
	}
	if _, err := os.Stat(ensureVenv); err != nil {
		fmt.Fprintf(stderr, "Python runtime bootstrap missing: %s\n", ensureVenv)
		return 2
	}
	profile := "runtime"
	if args[0] == "test" {
		profile = "test"
	}
	venv := exec.Command(ensureVenv, profile)
	venv.Stdin = stdin
	venv.Stderr = stderr
	venv.Env = os.Environ()
	pythonPath, err := venv.Output()
	if err != nil {
		fmt.Fprintf(stderr, "Python runtime bootstrap failed: %v\n", err)
		return 2
	}
	python := strings.TrimSpace(string(pythonPath))
	if python == "" {
		fmt.Fprintln(stderr, "Python runtime bootstrap returned empty interpreter")
		return 2
	}
	command := exec.Command(python, append([]string{orchestrator}, args...)...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "Python compatibility command failed: %v\n", err)
		return 2
	}
	return 0
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
	case "test", "dev", "debug", "init", "doctor", "skills", "update":
		return runPythonCommand(args, stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\nRun 'leedevkit' without arguments for usage.\n", args[0])
		return 1
	}
}
