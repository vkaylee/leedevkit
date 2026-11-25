package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/ownership"
)

// CLIRequest is the small normalized boundary needed by Go run execution.
type CLIRequest struct {
	Tool    string
	Args    []string
	Project string
	Pooler  bool
}

func ParseCLI(args []string) (CLIRequest, error) {
	if len(args) == 0 || args[0] != "run" {
		return CLIRequest{}, fmt.Errorf("usage: leedevkit run [--project PROJECT] [--pooler] TOOL [args...]")
	}
	request := CLIRequest{}
	index := 1
	for index < len(args) {
		switch args[index] {
		case "--project":
			if index+1 >= len(args) {
				return CLIRequest{}, fmt.Errorf("--project requires a value")
			}
			request.Project = args[index+1]
			index += 2
		case "--pooler":
			request.Pooler = true
			index++
		default:
			request.Tool = args[index]
			request.Args = append([]string(nil), args[index+1:]...)
			if len(request.Args) > 0 && request.Args[0] == "--" {
				request.Args = request.Args[1:]
			}
			if request.Tool == "" {
				return CLIRequest{}, fmt.Errorf("run tool is required")
			}
			return request, nil
		}
	}
	return CLIRequest{}, fmt.Errorf("run tool is required")
}

func ResolveContext(request CLIRequest, environ map[string]string, cwd string) (Request, error) {
	projectCfg, _ := config.Load(filepath.Join(cwd, "leedevkit.toml"))
	project := request.Project
	owned := false
	dev := false
	if project == "" {
		if envProj := strings.TrimSpace(environ["LEEDEVKIT_TEST_PROJECT"]); envProj != "" {
			project = envProj
			owned = !ownership.IsLocked(project)
		} else if projectCfg.Namespace != "" {
			project = projectCfg.Namespace + "-test"
			owned = !ownership.IsLocked(project)
		} else {
			generated, err := ownership.NewProjectName("leedevkit-test")
			if err != nil {
				return Request{}, err
			}
			project = generated
			owned = true
		}
	} else if project == "dev" {
		if envDev := strings.TrimSpace(environ["LEEDEVKIT_DEV_PROJECT"]); envDev != "" {
			project = envDev
		} else if projectCfg.Namespace != "" {
			project = projectCfg.Namespace + "-dev"
		} else {
			project = "leedevkit-dev"
		}
		dev = true
	}
	compose := splitArgv(environ["DOCKER_COMPOSE_TOOL"])
	if len(compose) == 0 {
		compose = []string{"docker", "compose"}
	}
	service := map[string]string{"go": "go", "cargo": "apiserver", "diesel": "apiserver", "npm": "webdashboard"}[request.Tool]
	if service == "" {
		return Request{}, fmt.Errorf("unsupported run tool %q", request.Tool)
	}
	lang := map[string]string{"go": "go", "cargo": "rust", "diesel": "rust", "npm": "typescript"}[request.Tool]
	workdir := ""
	if svc, ok := projectCfg.ServiceForLanguage(lang); ok && svc.Name != "" {
		service = svc.Name
		if svc.Workdir != "" {
			workdir = filepath.ToSlash(filepath.Join("/workspace", svc.Workdir))
		}
	}
	mode := map[string]string{"go": "go", "cargo": "api", "diesel": "api", "npm": "web"}[request.Tool]
	file := environ["LEEDEVKIT_COMPOSE_FILE_"+strings.ToUpper(mode)]
	if file == "" {
		file = filepath.Join(cwd, ".compose", "docker-compose.test.yml")
	}
	return Request{
		Compose: compose, Project: project, Files: []string{file}, Tool: request.Tool,
		Args: request.Args, Service: service, Workdir: workdir, Dev: dev, Pooler: request.Pooler,
		Running: false, Owned: owned,
	}, nil
}

func splitArgv(value string) []string { return strings.Fields(value) }

// Environment helper keeps CLI tests independent from process globals.
func currentEnvironment() map[string]string {
	result := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}
