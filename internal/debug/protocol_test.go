package debug

import (
	"encoding/json"
	"strings"
	"testing"
)

func protocolFixture(t *testing.T) Server {
	t.Helper()
	return NewServer(runtimeFixture(t))
}

func TestProtocolListsAndInitializes(t *testing.T) {
	server := protocolFixture(t)
	initialized := server.Handle(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"})
	if initialized["error"] != nil {
		t.Fatalf("initialize = %#v", initialized)
	}
	listed := server.Handle(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	if listed["error"] != nil {
		t.Fatalf("tools/list = %#v", listed)
	}
}

func TestProtocolCallsMetadataAndRejectsUnknownTool(t *testing.T) {
	server := protocolFixture(t)
	response := server.Handle(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "debug_environment", "arguments": map[string]any{}},
	})
	if response["error"] != nil {
		t.Fatalf("metadata = %#v", response)
	}
	unknown := server.Handle(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "unknown", "arguments": map[string]any{}},
	})
	if unknown["error"] == nil {
		t.Fatal("unknown tool accepted")
	}
}

func TestProtocolBoundsArgumentsAndRequests(t *testing.T) {
	server := protocolFixture(t)
	long := strings.Repeat("x", MaxProtocolString+1)
	response := server.Handle(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "debug_environment", "arguments": map[string]any{"extra": long}},
	})
	if response["error"] == nil {
		t.Fatal("oversized argument accepted")
	}
	if _, err := EncodeRequest(map[string]any{"jsonrpc": "2.0", "method": "ping"}); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRequest([]byte("{")); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}

func TestProtocolResourcesAndPrompt(t *testing.T) {
	server := protocolFixture(t)
	for _, request := range []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "resources/list"},
		{"jsonrpc": "2.0", "id": 2, "method": "resources/read", "params": map[string]any{"uri": "debug://safety"}},
		{"jsonrpc": "2.0", "id": 3, "method": "prompts/list"},
		{"jsonrpc": "2.0", "id": 4, "method": "prompts/get", "params": map[string]any{"name": "debug_incident", "arguments": map[string]any{"symptom": "slow"}}},
	} {
		if response := server.Handle(request); response["error"] != nil {
			t.Fatalf("request = %#v, response = %#v", request, response)
		}
	}
}

func TestProtocolServeProcessesJSONL(t *testing.T) {
	server := protocolFixture(t)
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n")
	var output strings.Builder
	if err := server.Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output.String())), &response); err != nil || response["result"] == nil {
		t.Fatalf("output = %q, err = %v", output.String(), err)
	}
}

func TestProtocolServeRejectsOversizedLine(t *testing.T) {
	server := protocolFixture(t)
	input := strings.NewReader(strings.Repeat("x", MaxRequestBytes+1) + "\n")
	var output strings.Builder
	if err := server.Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "request exceeds size limit") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestProtocolJSONRoundTrip(t *testing.T) {
	data, err := EncodeRequest(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil || decoded["method"] != "ping" {
		t.Fatalf("decoded = %#v, err = %v", decoded, err)
	}
}
