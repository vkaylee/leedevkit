package debug

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProtocolDispatchRejectsInvalidToolsAndArguments(t *testing.T) {
	server := protocolFixture(t)
	for _, params := range []any{
		nil,
		map[string]any{},
		map[string]any{"name": "service_health", "arguments": map[string]any{"service": "unknown", "path": "/health"}},
		map[string]any{"name": "http_get", "arguments": map[string]any{"service": "unknown", "path": "/"}},
		map[string]any{"name": "recent_logs", "arguments": map[string]any{"service": "unknown"}},
		map[string]any{"name": "database_query", "arguments": map[string]any{"template": "unknown"}},
		map[string]any{"name": "debug_environment", "arguments": map[string]any{strings.Repeat("k", 129): "x"}},
	} {
		response := server.Handle(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params})
		if response["error"] == nil {
			t.Errorf("accepted params %#v: %#v", params, response)
		}
	}
}

func TestProtocolServeRecoversFromMalformedInputAndIgnoresNotification(t *testing.T) {
	server := protocolFixture(t)
	input := "{\n" + `{"jsonrpc":"2.0","method":"ping"}` + "\n" + `{"jsonrpc":"2.0","id":3,"method":"ping"}` + "\n"
	var output strings.Builder
	if err := server.Serve(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses = %q", output.String())
	}
	var first, last map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first["error"] == nil {
		t.Fatalf("malformed response = %q, %v", lines[0], err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &last); err != nil || last["result"] == nil || last["id"] != float64(3) {
		t.Fatalf("ping response = %q, %v", lines[1], err)
	}
}
