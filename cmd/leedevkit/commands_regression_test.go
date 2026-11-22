package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectCommandsUseConfiguredProvider(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "engine")
	logPath := filepath.Join(root, "commands.log")
	script := "#!/bin/sh\nprintf '%s\\n' '---' \"$@\" >> \"$COMMAND_LOG\"\nexit ${ENGINE_EXIT:-0}\n"
	if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "[container]\nprovider = 'fixture'\n[container.providers.fixture]\ncli = ['" + engine + "']\ncompose = ['" + engine + "']\n" + `[services.backend]
lang = 'go'
[services.api]
lang = 'rust'
[services.frontend]
lang = 'web'
[quality.gates.generate]
phase = 'generated'
command = ['generate', 'check']
fix_command = ['generate', 'fix']
working_dir = 'backend'
required = true
[quality.go]
generated = ['generate']
`
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	t.Setenv("COMMAND_LOG", logPath)
	t.Setenv("COMPOSE_PROJECT_NAME", "fixture-project")

	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"coverage", []string{"coverage", "--json", "--pattern", "TestExample"}, []string{"exec\n-T\nbackend", "go test -json \"$@\"", "-run=TestExample"}},
		{"generated", []string{"generated", "--fix"}, []string{"exec\n-T\nbackend\n-w\n/workspace/backend\ngenerate\nfix"}},
		{"sync", []string{"sync:api"}, []string{"exec\n-T\napi", "exec\n-T\nfrontend"}},
		{"migration", []string{"migrate:status"}, []string{"fixture-project", "--entrypoint\ndiesel\napi\nmigration\nlist"}},
		{"query", []string{"db:query", "select 1", "--json"}, []string{"fixture-project-db", "select 1", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(logPath, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, strings.NewReader(""), &stdout, &stderr); code != 0 {
				t.Fatalf("exit=%d stderr=%q", code, stderr.String())
			}
			got, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(string(got), want) {
					t.Errorf("argv missing %q: %s", want, got)
				}
			}
		})
	}

	t.Run("engine failure stops sync", func(t *testing.T) {
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ENGINE_EXIT", "7")
		var stdout, stderr bytes.Buffer
		if code := run([]string{"sync:api"}, strings.NewReader(""), &stdout, &stderr); code != 7 {
			t.Fatalf("exit=%d stderr=%q", code, stderr.String())
		}
		got, err := os.ReadFile(logPath)
		if err != nil || strings.Count(string(got), "---\n") != 1 {
			t.Fatalf("commands=%q error=%v", got, err)
		}
	})
	for _, args := range [][]string{{"coverage", "--pattern"}, {"coverage", "--unknown"}, {"migrate:run", "extra"}, {"db:query"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 2 || stderr.Len() == 0 {
			t.Errorf("args=%q exit=%d stderr=%q", args, code, stderr.String())
		}
	}
}
