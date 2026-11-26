package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadProjectReadsServicesTargetsAndTestSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	content := `[services.go]
lang = "go"
workdir = "backend"
[services.web]
lang = "typescript"
[targets]
go = ["go"]
all = ["go", "web"]
[test]
build_policy = "phase"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Services["go"].Language != "go" || got.Services["go"].Workdir != "backend" {
		t.Fatalf("services = %#v", got.Services)
	}
	if !reflect.DeepEqual(got.Targets["all"], []string{"go", "web"}) {
		t.Fatalf("targets = %#v", got.Targets)
	}
	if got.BuildPolicy != "phase" {
		t.Fatalf("build policy = %q", got.BuildPolicy)
	}
}

func TestLoadMissingProjectUsesDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got.BuildPolicy != "once" {
		t.Fatalf("build policy = %q", got.BuildPolicy)
	}
}

func TestLoadIgnoresLegacyServiceImageAndComposeFileKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	content := `[services.go]
lang = "go"
dockerfile = "container/go/Dockerfile"
compose_file = "container/go/docker-compose.test.yml"
image = "project-go"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("legacy keys rejected: %v", err)
	}
	if got.Services["go"].Dockerfile != "container/go/Dockerfile" {
		t.Fatalf("service = %#v", got.Services["go"])
	}
}

func TestLoadNamespaceDefaultIntegrationAndExplicitGateOverrides(t *testing.T) {
	t.Run("default go integration gate from int-api dependency", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "leedevkit.toml")
		content := `[project]
name = "Shop"
namespace = "shop"
languages = ["go", "typescript"]

[services.apiserver]
lang = "go"
go = true
workdir = "apiserver"
ports = [8080]
health_check = "/health"

[targets]
api = ["apiserver"]
all = ["apiserver"]

[test.dependencies]
int-api = ["postgres", "redis", "apiserver"]
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Namespace != "shop" || got.Name != "Shop" {
			t.Fatalf("project metadata = %q / %q", got.Name, got.Namespace)
		}
		intGates := got.GatesForService("apiserver", "integration")
		if len(intGates) != 1 || !reflect.DeepEqual(intGates[0].Command, []string{"go", "test", "-tags=integration", "./..."}) {
			t.Fatalf("default intGates = %#v", intGates)
		}
	})

	t.Run("explicit [test.gates.int-api] override", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "leedevkit.toml")
		content := `[services.apiserver]
lang = "go"
[targets]
api = ["apiserver"]
[test.dependencies]
int-api = ["postgres", "apiserver"]
[test.gates.int-api]
command = ["go", "test", "-tags=integration", "-count=1", "./cmd/..."]
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		intGates := got.GatesForService("apiserver", "integration")
		if len(intGates) != 1 || !reflect.DeepEqual(intGates[0].Command, []string{"go", "test", "-tags=integration", "-count=1", "./cmd/..."}) {
			t.Fatalf("override intGates = %#v", intGates)
		}
		if len(got.GatesForService("apiserver", "unit")) != 1 {
			t.Fatalf("unit default lost: %#v", got.GatesForService("apiserver", "unit"))
		}
	})

	t.Run("explicit [services.apiserver.test] single and multi command overrides", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "leedevkit.toml")
		content := `[services.apiserver]
go = true
[services.apiserver.test]
lint = [["gofmt", "-l", "."], ["golangci-lint", "run"]]
integration = ["go", "test", "-tags=integration", "-race", "./..."]
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Services["apiserver"].Language != "go" {
			t.Fatalf("go=true did not infer lang=go: %#v", got.Services["apiserver"])
		}
		if lint := got.GatesForService("apiserver", "lint"); len(lint) != 2 || lint[1].Command[0] != "golangci-lint" {
			t.Fatalf("lint overrides = %#v", lint)
		}
		if integ := got.GatesForService("apiserver", "integration"); len(integ) != 1 || integ[0].Command[3] != "-race" {
			t.Fatalf("integration override = %#v", integ)
		}
	})

	t.Run("integration gate inherits default command when only env/setup_command/required_env are set", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "leedevkit.toml")
		content := `[services.apiserver]
lang = "go"
[targets]
api = ["apiserver"]
[test.dependencies]
int-api = ["postgres", "apiserver"]
[test.env]
SHARED_FLAG = "base"
APP_DB_INTEGRATION = "0"
[test.gates.int-api]
env = { APP_DB_INTEGRATION = "1" }
required_env = ["APP_DB_INTEGRATION", "DATABASE_URL"]
setup_command = ["go", "run", "./cmd/migrate", "up"]
`
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		intGates := got.GatesForService("apiserver", "integration")
		if len(intGates) != 1 {
			t.Fatalf("intGates = %#v", intGates)
		}
		g := intGates[0]
		if !reflect.DeepEqual(g.Command, []string{"go", "test", "-tags=integration", "./..."}) {
			t.Fatalf("command = %#v", g.Command)
		}
		if !reflect.DeepEqual(g.StructuredCommand, []string{"go", "test", "-json", "-tags=integration", "./..."}) {
			t.Fatalf("structured command = %#v", g.StructuredCommand)
		}
		if g.Env["APP_DB_INTEGRATION"] != "1" || g.Env["SHARED_FLAG"] != "base" {
			t.Fatalf("merged env = %#v", g.Env)
		}
		if !reflect.DeepEqual(g.RequiredEnv, []string{"APP_DB_INTEGRATION", "DATABASE_URL"}) {
			t.Fatalf("required_env = %#v", g.RequiredEnv)
		}
		if !reflect.DeepEqual(g.SetupCommand, []string{"go", "run", "./cmd/migrate", "up"}) {
			t.Fatalf("setup_command = %#v", g.SetupCommand)
		}
		unitGates := got.GatesForService("apiserver", "unit")
		if len(unitGates) != 1 || unitGates[0].Env["APP_DB_INTEGRATION"] != "0" {
			t.Fatalf("unit gate env should get base test.env without int-api override: %#v", unitGates)
		}
	})

	t.Run("rejects invalid namespace, empty dependencies, and bad gate definitions", func(t *testing.T) {
		for _, snippet := range []string{
			"[project]\nnamespace = '../bad'\n",
			"[test.dependencies]\nint-api = []\n",
			"[test.env]\n\"BAD=KEY\" = '1'\n",
			"[test.gates.unknown_name]\ncommand = ['go', 'test']\n",
			"[test.gates.int-api]\nenv = { \"BAD KEY\" = '1' }\n",
			"[test.gates.int-api]\nrequired_env = ['BAD=NAME']\n",
			"[services.api]\nlang = 'go'\n[services.api.test]\nintegration = []\n",
		} {
			path := filepath.Join(t.TempDir(), "bad.toml")
			if err := os.WriteFile(path, []byte(snippet), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatalf("expected error for snippet:\n%s", snippet)
			}
		}
	})
}

func TestLoadPlaywrightMCPConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leedevkit.toml")
	content := `[mcp.playwright]
endpoint = "http://caddy"
login_path = "/signin"
network = "leedevkit-dev_dev_frontend"
viewports = "config/ui-viewports.json"
accounts_doc = "docs/dev-accounts.md"
image = "custom/playwright:1"
startup_timeout = 20
request_timeout = 60
session_idle_timeout = 600
extra_args = ["--console-level=warning"]
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	pw := got.PlaywrightMCP
	if pw.Endpoint != "http://caddy" || pw.LoginPath != "/signin" || pw.Network != "leedevkit-dev_dev_frontend" ||
		pw.Viewports != "config/ui-viewports.json" || pw.AccountsDoc != "docs/dev-accounts.md" || pw.Image != "custom/playwright:1" ||
		pw.StartupTimeout != 20 || pw.RequestTimeout != 60 || pw.SessionIdleTimeout != 600 ||
		!reflect.DeepEqual(pw.ExtraArgs, []string{"--console-level=warning"}) {
		t.Fatalf("unexpected playwright config: %#v", pw)
	}

	for _, bad := range []string{
		"[mcp.playwright]\nendpoint = 'ftp://caddy'\n",
		"[mcp.playwright]\nendpoint = 'http://user:pass@caddy'\n",
		"[mcp.playwright]\nlogin_path = 'login'\n",
		"[mcp.playwright]\naccounts_doc = '../secrets.md'\n",
		"[mcp.playwright]\naccounts_doc = '/etc/passwd'\n",
		"[mcp.playwright]\nstartup_timeout = 999\n",
		"[mcp.playwright]\nrequest_timeout = -1\n",
		"[mcp.playwright]\nsession_idle_timeout = 9999\n",
	} {
		badPath := filepath.Join(t.TempDir(), "bad.toml")
		if err := os.WriteFile(badPath, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(badPath); err == nil {
			t.Fatalf("expected validation error for:\n%s", bad)
		}
	}
}
