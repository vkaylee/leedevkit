package run

import (
	"fmt"
	"strings"
)

// Request is normalized run context. Go owns configuration and Compose
// discovery alongside argv planning and execution policy.
type Request struct {
	Compose  []string `json:"compose"`
	Project  string   `json:"project"`
	Profiles []string `json:"profiles"`
	Files    []string `json:"files"`
	Tool     string   `json:"tool"`
	Args     []string `json:"args"`
	Service  string   `json:"service"`
	Workdir  string   `json:"workdir"`
	Running  bool     `json:"running"`
	Dev      bool     `json:"dev"`
	Pooler   bool     `json:"pooler"`
	Owned    bool     `json:"owned"`
}

var supportedTools = map[string]bool{
	"cargo":  true,
	"diesel": true,
	"go":     true,
	"npm":    true,
}

// Plan builds Compose argv without executing Compose or a shell.
func Plan(request Request) ([]string, error) {
	if len(request.Compose) == 0 || request.Project == "" || request.Service == "" {
		return nil, fmt.Errorf("compose, project, and service are required")
	}
	if !supportedTools[request.Tool] {
		return nil, fmt.Errorf("unsupported run tool %q", request.Tool)
	}
	if err := validateArgs(request.Args); err != nil {
		return nil, err
	}

	profiles := append([]string(nil), request.Profiles...)
	providedProfiles := len(profiles) > 0
	if len(profiles) == 0 {
		profiles = defaultProfiles(request.Tool, request.Args, request.Pooler)
	}
	if providedProfiles && request.Tool == "cargo" && cargoNeedsDB(request.Args) {
		profiles = append(profiles, "--profile", "infra-db", "--profile", "infra-redis")
		if request.Pooler {
			profiles = append(profiles, "--profile", "infra-pooler")
		}
	}
	args := append([]string{}, request.Compose...)
	args = append(args, "-p", request.Project)
	args = append(args, profiles...)
	for _, file := range request.Files {
		args = append(args, "-f", file)
	}

	switch request.Tool {
	case "go":
		if request.Running {
			args = append(args, "exec", "-T", request.Service, "go")
		} else {
			args = append(args, "run", "-T", "--rm", "--no-deps", "--entrypoint", "go", request.Service)
		}
		return append(args, request.Args...), nil
	case "cargo":
		needsDB := cargoNeedsDB(request.Args)
		if request.Running {
			args = append(args, "exec", "-T")
		} else {
			args = append(args, "run", "-T", "--rm")
			if !needsDB {
				args = append(args, "--no-deps")
			}
		}
		if request.Workdir != "" {
			args = append(args, "--workdir", request.Workdir)
		}
		if request.Running {
			args = append(args, request.Service, "cargo")
		} else {
			args = append(args, "--entrypoint", "cargo", request.Service)
		}
		toolArgs := append([]string(nil), request.Args...)
		if len(toolArgs) > 0 && toolArgs[0] == "test" {
			toolArgs = append([]string{"nextest", "run", "--test-threads", "4"}, toolArgs[1:]...)
		}
		return append(args, toolArgs...), nil
	case "npm":
		toolArgs := append([]string(nil), request.Args...)
		if len(toolArgs) >= 2 && toolArgs[0] == "run" && toolArgs[1] == "typecheck" {
			toolArgs[1] = "type-check"
		}
		executable := "bun"
		if len(toolArgs) > 0 && (toolArgs[0] == "bun" || toolArgs[0] == "npm" || toolArgs[0] == "bash" || toolArgs[0] == "sh") {
			executable, toolArgs = toolArgs[0], toolArgs[1:]
		}
		standalone := len(request.Args) > 0 && (request.Args[0] == "--version" || request.Args[0] == "--help" || request.Args[0] == "-v" || request.Args[0] == "-h")
		if request.Running {
			args = append(args, "exec", "-T", request.Service, executable)
		} else {
			args = append(args, "run", "-T", "--rm")
			if standalone {
				args = append(args, "--no-deps")
			}
			args = append(args, "--entrypoint", executable, request.Service)
		}
		return append(args, toolArgs...), nil
	case "diesel":
		if request.Running {
			args = append(args, "exec", "-T", request.Service, "diesel")
		} else {
			args = append(args, "run", "-T", "--rm", "--entrypoint", "diesel", request.Service)
		}
		return append(args, request.Args...), nil
	}
	panic("unreachable")
}

func defaultProfiles(tool string, toolArgs []string, pooler bool) []string {
	switch tool {
	case "go":
		return []string{"--profile", "go"}
	case "npm":
		return []string{"--profile", "frontend"}
	case "cargo", "diesel":
		profiles := []string{"--profile", "int-api"}
		if cargoNeedsDB(toolArgs) || tool == "diesel" {
			profiles = append(profiles, "--profile", "infra-db", "--profile", "infra-redis")
			if pooler {
				profiles = append(profiles, "--profile", "infra-pooler")
			}
		}
		return profiles
	default:
		return nil
	}
}

func cargoNeedsDB(args []string) bool {
	return len(args) > 0 && (args[0] == "run" || args[0] == "test" || args[0] == "nextest")
}

func validateArgs(args []string) error {
	for _, arg := range args {
		for _, character := range arg {
			if character < 32 && character != '\t' {
				return fmt.Errorf("unsafe run argument: control character U+%04X", character)
			}
		}
		if strings.Count(arg, "\"")%2 != 0 || strings.Count(arg, "'")%2 != 0 {
			return fmt.Errorf("unsafe run argument: unmatched quote")
		}
		for _, pattern := range []string{"$(", "`", "${", "#{", "<(", ")>", "\\x", "<<<", "<<EOF", "<<-EOF", "\n", "\r"} {
			if strings.Contains(arg, pattern) {
				return fmt.Errorf("unsafe run argument: forbidden pattern %q", pattern)
			}
		}
	}
	if len(args) >= 2 && (args[0] == "node" || args[0] == "ruby" || args[0] == "perl" || args[0] == "php") && (args[1] == "-c" || args[1] == "-e" || args[1] == "-r") {
		return fmt.Errorf("unsafe run argument: inline interpreter execution")
	}
	return nil
}
