package provider

import (
	"reflect"
	"strings"
	"testing"
)

func TestProtocolRedactsNestedCredentialsAndPreservesInput(t *testing.T) {
	settings := map[string]any{
		"nested":  []any{map[string]any{"api_key": "secret", "url": "https://user:password@example.test/path"}},
		"visible": "safe",
	}
	input := ProtocolProvider{Name: "fixture", CLI: []string{"tool"}, Compose: []string{"tool", "compose"}, Settings: settings, Capabilities: []string{"up", "down", "up"}}
	got, err := ValidateProtocol(input)
	if err != nil {
		t.Fatal(err)
	}
	values := got["settings"].(map[string]any)
	nested := values["nested"].([]any)[0].(map[string]any)
	if nested["api_key"] != "<redacted>" || nested["url"] != "https://example.test/path" || values["visible"] != "safe" {
		t.Fatalf("redacted settings = %#v", values)
	}
	if settings["nested"].([]any)[0].(map[string]any)["api_key"] != "secret" {
		t.Fatal("redaction mutated input")
	}
	if !reflect.DeepEqual(got["capabilities"], []string{"down", "up"}) {
		t.Fatalf("capabilities = %#v", got["capabilities"])
	}
}

func TestProtocolRejectsInvalidProvider(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input ProtocolProvider
		want  string
	}{
		{"empty name", ProtocolProvider{}, "name"},
		{"empty cli", ProtocolProvider{Name: "p", Compose: []string{"tool"}}, "cli"},
		{"blank argument", ProtocolProvider{Name: "p", CLI: []string{"tool", "  "}, Compose: []string{"tool"}}, "cli"},
		{"empty compose", ProtocolProvider{Name: "p", CLI: []string{"tool"}}, "compose"},
		{"unknown capability", ProtocolProvider{Name: "p", CLI: []string{"tool"}, Compose: []string{"tool"}, Capabilities: []string{"unknown"}}, "unknown capability"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateProtocol(tc.input); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
