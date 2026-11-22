package run

import (
	"reflect"
	"strings"
	"testing"
)

func TestPlanToolsAgainstExistingAndIsolatedServices(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request Request
		want    []string
	}{
		{"running go", Request{Tool: "go", Service: "go", Running: true, Args: []string{"version"}}, []string{"docker", "compose", "-p", "p", "--profile", "go", "exec", "-T", "go", "go", "version"}},
		{"diesel", Request{Tool: "diesel", Service: "api", Pooler: true, Args: []string{"migration", "list"}}, []string{"docker", "compose", "-p", "p", "--profile", "int-api", "--profile", "infra-db", "--profile", "infra-redis", "--profile", "infra-pooler", "run", "-T", "--rm", "--entrypoint", "diesel", "api", "migration", "list"}},
		{"npm help", Request{Tool: "npm", Service: "web", Args: []string{"--help"}}, []string{"docker", "compose", "-p", "p", "--profile", "frontend", "run", "-T", "--rm", "--no-deps", "--entrypoint", "bun", "web", "--help"}},
		{"cargo check", Request{Tool: "cargo", Service: "api", Running: true, Workdir: "/src", Args: []string{"check"}}, []string{"docker", "compose", "-p", "p", "--profile", "int-api", "exec", "-T", "--workdir", "/src", "api", "cargo", "check"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.request.Compose = []string{"docker", "compose"}
			tc.request.Project = "p"
			got, err := Plan(tc.request)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("plan = %q, error = %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestPlanRejectsUnsafeInputs(t *testing.T) {
	for _, arg := range []string{"line\nbreak", "'unmatched", "`id`", "${HOME}", "<(id)", "\\x41"} {
		_, err := Plan(Request{Compose: []string{"docker"}, Project: "p", Service: "go", Tool: "go", Args: []string{arg}})
		if err == nil || !strings.Contains(err.Error(), "unsafe run argument") {
			t.Errorf("argument %q error = %v", arg, err)
		}
	}
	_, err := Plan(Request{Compose: []string{"docker"}, Project: "p", Service: "web", Tool: "npm", Args: []string{"node", "-e", "x"}})
	if err == nil || !strings.Contains(err.Error(), "inline interpreter") {
		t.Fatalf("inline execution error = %v", err)
	}
}
