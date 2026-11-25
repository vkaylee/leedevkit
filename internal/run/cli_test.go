package run

import (
	"os"
	"testing"
)

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

func TestResolveContextUsesManifestNamespace(t *testing.T) {
	root := t.TempDir()
	manifest := "[project]\nnamespace = 'acme'\n[services.apiserver]\nlang = 'go'\nworkdir = 'apiserver'\n"
	if err := os.WriteFile(root+"/leedevkit.toml", []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	testReq, err := ResolveContext(CLIRequest{Tool: "go", Args: []string{"generate", "./..."}}, nil, root)
	if err != nil {
		t.Fatal(err)
	}
	if testReq.Project != "acme-test" || testReq.Service != "apiserver" || testReq.Workdir != "/workspace/apiserver" {
		t.Fatalf("testReq = %#v", testReq)
	}
	devReq, err := ResolveContext(CLIRequest{Tool: "go", Project: "dev", Args: []string{"version"}}, nil, root)
	if err != nil {
		t.Fatal(err)
	}
	if devReq.Project != "acme-dev" || !devReq.Dev {
		t.Fatalf("devReq = %#v", devReq)
	}
}
