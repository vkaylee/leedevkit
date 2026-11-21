package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runtimeFixture(t *testing.T) Environment {
	root := t.TempDir()
	logPath := filepath.Join(root, "api.log")
	if err := os.WriteFile(logPath, []byte("ok\npassword=hunter2\nerror request=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return Environment{
		Name: "fixture", ComposeProject: "fixture-project", ProjectRoot: root,
		Services:  map[string]Service{"api": {ID: "api", Host: "api", Port: 8080, Aliases: []string{"api"}, Health: []string{"/health"}, HTTPGet: []string{"/status"}, LogFiles: []string{"api.log"}}},
		Redaction: []string{"(?i)password=[^ ]+"}, Limits: map[string]int{"request_timeout_ms": 100, "max_bytes": 64, "max_log_lines": 2},
	}
}

func TestRuntimeMetadataAndApprovedPaths(t *testing.T) {
	runtime := NewRuntime(runtimeFixture(t))
	metadata := runtime.Metadata()
	if metadata["mode"] != "read-only" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if _, err := runtime.Health("api", "/not-approved"); err == nil {
		t.Fatal("unapproved health path accepted")
	}
	if _, err := runtime.HTTPGet("api", "/not-approved"); err == nil {
		t.Fatal("unapproved HTTP path accepted")
	}
}

func TestRuntimeReadsBoundedRedactedLogs(t *testing.T) {
	runtime := NewRuntime(runtimeFixture(t))
	result, err := runtime.RecentLogs("api", "api.log", "error")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "request=1") || strings.Contains(strings.Join(result.Lines, "\n"), "hunter2") {
		t.Fatalf("logs = %#v", result)
	}
}

func TestRuntimeRejectsUnsafeLogSource(t *testing.T) {
	environment := runtimeFixture(t)
	environment.Services["api"] = Service{ID: "api", LogFiles: []string{"../secret"}}
	if _, err := NewRuntime(environment).RecentLogs("api", "../secret", ""); err == nil {
		t.Fatal("unsafe log source accepted")
	}
}

func TestRuntimeReadOnlyDatabaseTemplate(t *testing.T) {
	environment := runtimeFixture(t)
	environment.Database = map[string]any{"enabled": true, "schemas": []string{"public"}, "templates": map[string]any{"health": "SELECT 1"}}
	runtime := NewRuntime(environment)
	result, err := runtime.DatabaseSchema()
	if err != nil || !result.MetadataOnly {
		t.Fatalf("schema = %#v, err = %v", result, err)
	}
	if _, err := runtime.DatabaseQuery("missing", nil); err == nil {
		t.Fatal("missing query template accepted")
	}
	if _, err := runtime.DatabaseQuery("health", nil); err == nil {
		t.Fatal("unconfigured database adapter should not execute query")
	}
}
