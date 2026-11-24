package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/config"
)

func TestTargetsCommandJSONUsesConfiguredData(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(`[project]
languages = ["go", "typescript"]
[services.api]
lang = "go"
[services.webdashboard]
lang = "typescript"
[targets]
all = ["backend", "web", "infra"]
backend = ["api"]
web = ["webdashboard"]
infra = ["infra"]
[test.dependencies]
unit-api = ["db", "api"]
int-api = ["db", "api"]
e2e-web = ["browser", "web"]
lint-api = ["lint", "api"]
lint-web = ["web"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"targets", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	var got struct {
		Targets map[string]struct {
			Targets   []string `json:"targets"`
			Languages []string `json:"languages"`
			Services  []string `json:"services"`
			Phases    []string `json:"phases"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; output=%q", err, stdout.String())
	}
	if len(got.Targets) != 6 || strings.Join(got.Targets["all"].Targets, ",") != "backend,web,infra" || strings.Join(got.Targets["all"].Services, ",") != "api,webdashboard" {
		t.Fatalf("targets = %#v", got.Targets)
	}
	if strings.Join(got.Targets["backend"].Languages, ",") != "go" || strings.Join(got.Targets["backend"].Services, ",") != "api" || strings.Join(got.Targets["backend"].Phases, ",") != "unit,integration,lint" {
		t.Fatalf("backend target = %#v", got.Targets["backend"])
	}
	if strings.Join(got.Targets["all"].Phases, ",") != "infrastructure,unit,integration,e2e,lint" {
		t.Fatalf("all phases = %#v", got.Targets["all"].Phases)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestTargetsIncludesDirectServiceTargets(t *testing.T) {
	project := config.Project{Services: map[string]config.Service{"api": {Language: "go"}}, Targets: map[string][]string{"all": {"api"}}}
	targets := describeTargets(project).Targets
	if entry, ok := targets["api"]; !ok || len(entry.Services) != 1 || entry.Services[0] != "api" {
		t.Fatalf("direct service target missing: %#v", targets)
	}
}

func TestTargetsCommandRejectsUnknownFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := printTargets([]string{"targets", "--other"}, &stdout, &stderr, config.Project{}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage: leedevkit targets [--json]") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
