package debug

import (
	"strings"
	"testing"
)

func TestDatabaseToolsRejectUnapprovedOrMutatingQueries(t *testing.T) {
	env := runtimeFixture(t)
	runtime := NewRuntime(env)
	if _, err := runtime.DatabaseSchema(); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("disabled schema error = %v", err)
	}
	env.Database = map[string]any{"enabled": true, "schemas": []any{"public", "audit"}, "templates": map[string]any{"write": "DELETE FROM users", "invalid": 42, "read": "SELECT 1"}}
	runtime = NewRuntime(env)
	if schema, err := runtime.DatabaseSchema(); err != nil || len(schema.Schemas) != 2 || schema.Schemas[0] != "public" || !schema.MetadataOnly {
		t.Fatalf("schema = %+v, %v", schema, err)
	}
	for _, name := range []string{"write", "invalid"} {
		if _, err := runtime.DatabaseQuery(name, nil); err == nil || !strings.Contains(err.Error(), "not read-only") {
			t.Errorf("query %q error = %v", name, err)
		}
	}
	if _, err := runtime.DatabaseQuery("read", nil); err == nil || !strings.Contains(err.Error(), "adapter not configured") {
		t.Fatalf("read query error = %v", err)
	}
}
