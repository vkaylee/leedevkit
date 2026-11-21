package dbcommands

import (
	"fmt"
	"path/filepath"
)

type DieselRequest struct {
	Compose     []string
	Project     string
	ComposeFile string
	Root        string
	WorkingDir  string
	Service     string
	Args        []string
}

func BuildDiesel(request DieselRequest) ([]string, error) {
	if len(request.Compose) == 0 || request.Project == "" || request.ComposeFile == "" || request.Root == "" || request.Service == "" || len(request.Args) == 0 {
		return nil, fmt.Errorf("compose, project, compose file, root, service, and diesel args are required")
	}
	relative, err := filepath.Rel(filepath.Clean(request.Root), filepath.Clean(request.WorkingDir))
	if err != nil || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return nil, fmt.Errorf("working directory must stay inside project")
	}
	workdir := "/workspace"
	if relative != "." {
		workdir += "/" + filepath.ToSlash(relative)
	}
	args := append([]string{}, request.Compose...)
	args = append(args, "-p", request.Project, "-f", request.ComposeFile, "run", "-T", "--rm", "--workdir", workdir, "--entrypoint", "diesel", request.Service)
	return append(args, request.Args...), nil
}

func MigrationArgs(action string) ([]string, error) {
	switch action {
	case "run":
		return []string{"migration", "run"}, nil
	case "revert":
		return []string{"migration", "revert"}, nil
	case "status":
		return []string{"migration", "list"}, nil
	default:
		return nil, fmt.Errorf("unsupported migration action %q", action)
	}
}
