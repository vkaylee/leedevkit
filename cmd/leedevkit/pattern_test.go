package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/gates"
)

func TestPatternGoGate(t *testing.T) {
	gate, err := patternGoGate(gates.Defaults(), `TestFoo; touch /tmp/no`)
	if err == nil || len(gate) != 0 {
		t.Fatal("subtest patterns must be rejected until matching is verified")
	}
	gate, err = patternGoGate(gates.Defaults(), `TestFoo; touch marker`)
	if err != nil || len(gate) != 1 || gate[0].Command[3] != `-run=TestFoo; touch marker` {
		t.Fatalf("gate=%#v err=%v", gate, err)
	}
	for _, pattern := range []string{"[", "", "TestFoo/sub"} {
		if _, err := patternGoGate(gates.Defaults(), pattern); err == nil {
			t.Errorf("accepted %q", pattern)
		}
	}
	conditional := gates.Defaults()
	conditional[2].Condition = "never"
	if _, err := patternGoGate(conditional, "TestFoo"); err == nil {
		t.Fatal("accepted gate that may not execute")
	}
	custom := append(gates.Defaults(), gates.Gate{Name: "custom", Phase: "unit", Command: []string{"true"}})
	if _, err := patternGoGate(custom, "TestFoo"); err == nil {
		t.Fatal("silently ignored custom unit gate")
	}
}

func TestDefaultGoJSONThroughCompose(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "compose-fixture")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$ARGV_LOG\"\nfor arg do\n if [ \"$arg\" = ps ]; then printf '[{\"Service\":\"backend\",\"State\":\"running\"}]\\n'; exit; fi\n if [ \"$arg\" = exec ]; then printf '%s\\n' '{\"Action\":\"output\",\"Package\":\"p\",\"Test\":\"TestBad\",\"Output\":\"bad_test.go:12: broken\\\\n\"}' '{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestBad\"}'; exit 1; fi\ndone\n"
	if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := "[container]\nprovider = 'fixture'\n[container.providers.fixture]\ncli = ['fixture-engine']\ncompose = ['" + engine + "']\n[services.backend]\nlang = 'go'\n[targets]\nall = ['go']\n"
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".compose"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	log := filepath.Join(root, "argv.log")
	t.Setenv("ARGV_LOG", log)
	var stdout, stderr bytes.Buffer
	code := run([]string{"test", "all", "--unit-only", "--json"}, strings.NewReader(""), &stdout, &stderr)
	var summary struct {
		Failures []struct {
			TestName string `json:"test_name"`
			Line     int    `json:"line"`
		} `json:"failures"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("stdout=%q stderr=%q: %v", stdout.String(), stderr.String(), err)
	}
	if code != 1 || len(summary.Failures) != 1 || summary.Failures[0].TestName != "TestBad" || summary.Failures[0].Line != 12 {
		t.Fatalf("exit=%d summary=%#v stderr=%q", code, summary, stderr.String())
	}
	args, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(args), "-json\n") {
		t.Fatalf("argv=%q err=%v", args, err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"test", "all", "--unit-only", "--json-stream", "--quiet"}, strings.NewReader(""), &stdout, &stderr)
	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte{'\n'})
	var finished struct {
		Event   string `json:"event"`
		Summary struct {
			Failures []struct {
				TestName string `json:"test_name"`
			} `json:"failures"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(lines[len(lines)-1], &finished); err != nil || code != 1 || finished.Event != "run_finished" || len(finished.Summary.Failures) != 1 || finished.Summary.Failures[0].TestName != "TestBad" {
		t.Fatalf("exit=%d event=%#v err=%v stderr=%q", code, finished, err, stderr.String())
	}
}

func TestPatternCLIThroughCompose(t *testing.T) {
	root := t.TempDir()
	engine := filepath.Join(root, "compose-fixture")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$ARGV_LOG\"\nfor arg do\n if [ \"$arg\" = ps ]; then printf '[{\"Service\":\"backend\",\"State\":\"running\"}]\\n'; exit; fi\n if [ \"$arg\" = exec ]; then printf '%s\\n' \"$TEST_EVENT\"; fi\ndone\n"
	if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "[container]\nprovider = 'fixture'\n[container.providers.fixture]\ncli = ['fixture-engine']\ncompose = ['" + engine + "']\n[services.backend]\nlang = 'go'\n[targets]\nall = ['go']\n"
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".compose"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".compose", "docker-compose.test.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("HOME", root)
	t.Setenv("LEEDEVKIT_CONTAINER_PROVIDER", "fixture")
	log := filepath.Join(root, "argv.log")
	t.Setenv("ARGV_LOG", log)
	for _, tc := range []struct {
		name, event string
		code        int
		status      string
	}{
		{"match", `{"Action":"run","Test":"TestFoo"}`, 0, "partial"},
		{"skip", `{"Action":"skip","Test":"TestFoo"}`, 0, "partial"},
		{"no-match", `{"Action":"pass","Package":"example"}`, 1, "failed"},
		{"invalid-output", `not-json`, 1, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_EVENT", tc.event)
			var stdout, stderr bytes.Buffer
			code := run([]string{"test", "all", "--pattern", `TestFoo; touch marker`, "--json"}, strings.NewReader(""), &stdout, &stderr)
			var summary struct {
				Status string `json:"status"`
				Full   bool   `json:"full_regression"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
				t.Fatalf("stdout=%q stderr=%q: %v", stdout.String(), stderr.String(), err)
			}
			if code != tc.code || summary.Status != tc.status || summary.Full {
				t.Fatalf("exit=%d summary=%#v stderr=%q", code, summary, stderr.String())
			}
		})
	}
	for _, conflict := range []string{"--lint-only", "--e2e-only", "--coverage", "--fix"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"test", "all", "--pattern", "TestFoo", conflict}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--pattern") {
			t.Errorf("%s: exit=%d stderr=%q", conflict, code, stderr.String())
		}
	}
	data, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(data), "-run=TestFoo; touch marker\n") {
		t.Fatalf("args=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "marker")); !os.IsNotExist(err) {
		t.Fatalf("pattern ran as shell: %v", err)
	}
}

func TestPatternWithRealGoTest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/fixture\n\ngo 1.24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixture_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestSelected(t *testing.T) {}\nfunc TestSkipped(t *testing.T) { t.Skip() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		pattern string
		matched bool
	}{{"TestSelected", true}, {"TestSkipped", true}, {"TestMissing", false}} {
		command := exec.Command("go", "test", "-json", "-run="+tc.pattern, "./...")
		command.Dir = root
		output, err := command.Output()
		if err != nil {
			t.Fatalf("go test %s: %v", tc.pattern, err)
		}
		matched, err := patternGoEvents(bytes.NewReader(output))
		if err != nil || matched != tc.matched {
			t.Errorf("%s: matched=%v err=%v", tc.pattern, matched, err)
		}
	}
}

func TestPatternGoEvents(t *testing.T) {
	for _, tc := range []struct {
		output  string
		match   bool
		invalid bool
	}{
		{`{"Action":"run","Test":"TestFoo"}`, true, false},
		{`{"Action":"skip","Test":"TestFoo"}`, true, false},
		{`{"Action":"run","Package":"example"}`, false, false},
		{`not-json`, false, true},
	} {
		matched, err := patternGoEvents(strings.NewReader(tc.output))
		if matched != tc.match || (err != nil) != tc.invalid {
			t.Errorf("%q: matched=%v err=%v", tc.output, matched, err)
		}
	}
}
