package run

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseCLIRejectsIncompleteRequests(t *testing.T) {
	for _, args := range [][]string{nil, {"test"}, {"run"}, {"run", "--project"}, {"run", ""}} {
		if _, err := ParseCLI(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}

func TestResolveContextRespectsExplicitProjectAndComposeFile(t *testing.T) {
	request, err := ResolveContext(CLIRequest{Tool: "diesel", Project: "other", Args: []string{"migration", "list"}}, map[string]string{
		"DOCKER_COMPOSE_TOOL":        "podman compose",
		"LEEDEVKIT_COMPOSE_FILE_API": "/custom.yml",
	}, "/project")
	if err != nil {
		t.Fatal(err)
	}
	if request.Project != "other" || request.Owned || request.Dev || request.Service != "apiserver" || !reflect.DeepEqual(request.Compose, []string{"podman", "compose"}) || !reflect.DeepEqual(request.Files, []string{"/custom.yml"}) {
		t.Fatalf("request = %+v", request)
	}
	fallback, err := ResolveContext(CLIRequest{Tool: "go", Project: "p"}, nil, "/project")
	if err != nil || !reflect.DeepEqual(fallback.Files, []string{filepath.Join("/project", ".compose", "docker-compose.test.yml")}) || !reflect.DeepEqual(fallback.Compose, []string{"docker", "compose"}) {
		t.Fatalf("fallback = %+v, error = %v", fallback, err)
	}
	if _, err := ResolveContext(CLIRequest{Tool: "unknown", Project: "p"}, nil, "/project"); err == nil || !strings.Contains(err.Error(), "unsupported run tool") {
		t.Fatalf("unknown tool error = %v", err)
	}
}
