package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestProjectMCPStdioListsAndCallsTools(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/leedevkit.toml", []byte("[targets]\nall = [\"go\", \"web\"]\nzeta = [\"go\"]\nalpha = [\"web\"]\n[test.dependencies]\nunit-zeta = [\"go\"]\nunit-alpha = [\"web\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader("" +
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_targets","arguments":{}}}` + "\n")
	var output, diagnostics bytes.Buffer
	if err := serveProjectMCP(input, &output, &diagnostics, root); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(&output)
	var responses []map[string]any
	for scanner.Scan() {
		var response map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		responses = append(responses, response)
	}
	if len(responses) != 3 {
		t.Fatalf("got %d responses: %s", len(responses), output.String())
	}
	initialize := responses[0]["result"].(map[string]any)
	if initialize["protocolVersion"] != "2024-11-05" {
		t.Fatalf("initialize result = %#v", initialize)
	}
	tools := responses[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("got %d tools", len(tools))
	}
	gotNames := []string{}
	for _, raw := range tools {
		gotNames = append(gotNames, raw.(map[string]any)["name"].(string))
	}
	if strings.Join(gotNames, ",") != "run_test,get_project_status,list_targets" {
		t.Fatalf("tool names = %v", gotNames)
	}
	callResult := responses[2]["result"].(map[string]any)
	if callResult["isError"] != false || len(callResult["content"].([]any)) != 1 {
		t.Fatalf("MCP tool result = %#v", callResult)
	}
	var listed map[string]any
	if err := json.Unmarshal([]byte(callResult["content"].([]any)[0].(map[string]any)["text"].(string)), &listed); err != nil {
		t.Fatal(err)
	}
	names := listed["targets"].(map[string]any)
	if _, ok := names["alpha"]; !ok {
		t.Fatalf("targets = %#v", names)
	}
	if _, ok := names["zeta"]; !ok {
		t.Fatalf("targets = %#v", names)
	}
}

func TestProjectMCPRejectsInvalidRequestID(t *testing.T) {
	for _, id := range []string{`{"invalid":true}`, `[]`, `true`, `"42"`} {
		var output bytes.Buffer
		input := strings.NewReader(`{"jsonrpc":"2.0","id":` + id + `,"method":"ping"}` + "\n")
		if err := serveProjectMCP(input, &output, &bytes.Buffer{}, t.TempDir()); err != nil {
			t.Fatal(err)
		}
		var response struct {
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(output.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if id == `"42"` && response.Error.Code == 0 {
			continue
		}
		if response.Error.Code != -32600 {
			t.Fatalf("id=%s response=%s", id, output.String())
		}
	}
}

func TestProjectMCPRejectsUnsafeFlagsAndArguments(t *testing.T) {
	for _, input := range []string{
		`{"name":"run_test","arguments":{"target":"all","flags":["--arbitrary"]}}`,
		`{"name":"run_test","arguments":{"target":"all;whoami"}}`,
		`{"name":"run_test","arguments":{"target":"all","unknown":true}}`,
	} {
		_, rpcErr := callProjectMCPTool(json.RawMessage(input), t.TempDir(), &bytes.Buffer{})
		if rpcErr == nil || rpcErr["code"] != -32602 {
			t.Errorf("input %s error = %#v", input, rpcErr)
		}
	}
}
