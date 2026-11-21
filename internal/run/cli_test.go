package run

import "testing"

func TestParseCLIProjectPoolerAndSeparator(t *testing.T) {
	request, err := ParseCLI([]string{"run", "--project", "dev", "--pooler", "cargo", "--", "test"})
	if err != nil {
		t.Fatal(err)
	}
	if request.Project != "dev" || !request.Pooler || request.Tool != "cargo" || len(request.Args) != 1 || request.Args[0] != "test" {
		t.Fatalf("request = %#v", request)
	}
}

func TestResolveContextCreatesOwnedProject(t *testing.T) {
	request, err := ResolveContext(CLIRequest{Tool: "go", Args: []string{"version"}}, map[string]string{"DOCKER_COMPOSE_TOOL": "docker compose"}, "/project")
	if err != nil {
		t.Fatal(err)
	}
	if !request.Owned || request.Project == "" || request.Service != "go" {
		t.Fatalf("request = %#v", request)
	}
}

func TestResolveContextExternalDevProject(t *testing.T) {
	request, err := ResolveContext(CLIRequest{Tool: "npm", Project: "dev"}, map[string]string{"DOCKER_COMPOSE_TOOL": "podman-compose"}, "/project")
	if err != nil {
		t.Fatal(err)
	}
	if request.Owned || !request.Dev || request.Project != "leedevkit-dev" {
		t.Fatalf("request = %#v", request)
	}
}
