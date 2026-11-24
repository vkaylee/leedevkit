package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/process"
)

const mcpMaxRequestBytes = 256 * 1024

func serveProjectMCP(input io.Reader, output, diagnostics io.Writer, root string) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), mcpMaxRequestBytes)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			if err := encoder.Encode(mcpError(nil, -32700, "Parse error")); err != nil {
				return err
			}
			continue
		}
		if len(request.ID) > 0 && !validMCPRequestID(request.ID) {
			if err := encoder.Encode(mcpError(nil, -32600, "Invalid Request")); err != nil {
				return err
			}
			continue
		}
		if request.JSONRPC != "2.0" || request.Method == "" {
			if err := encoder.Encode(mcpError(requestID(request.ID), -32600, "Invalid Request")); err != nil {
				return err
			}
			continue
		}
		if len(request.ID) == 0 || string(request.ID) == "null" {
			if strings.HasPrefix(request.Method, "notifications/") {
				continue
			}
			continue
		}
		var result any
		var rpcErr map[string]any
		switch request.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "leedevkit", "version": "1.0"}}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			result = map[string]any{"tools": projectMCPTools()}
		case "tools/call":
			result, rpcErr = callProjectMCPTool(request.Params, root, diagnostics)
		default:
			rpcErr = map[string]any{"code": -32601, "message": "Method not found"}
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if rpcErr != nil {
			response["error"] = rpcErr
		} else {
			response["result"] = result
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func validMCPRequestID(id json.RawMessage) bool {
	if string(id) == "null" {
		return true
	}
	var text string
	if json.Unmarshal(id, &text) == nil {
		return true
	}
	var number json.Number
	return json.Unmarshal(id, &number) == nil
}

func requestID(id json.RawMessage) any {
	if len(id) == 0 || string(id) == "null" {
		return nil
	}
	return id
}

func mcpError(id any, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

func projectMCPTools() []map[string]any {
	return []map[string]any{
		{"name": "run_test", "description": "Run configured LeeDevKit tests for target with optional safe CLI flags", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"target": map[string]any{"type": "string"}, "flags": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"--lint-only", "--unit-only", "--e2e-only", "--coverage", "--skip-lint", "--skip-e2e", "--quiet", "--failed-only", "--compact"}}}}, "required": []string{"target"}, "additionalProperties": false}},
		{"name": "get_project_status", "description": "Get project root, manifest availability, and Git working-tree status", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
		{"name": "list_targets", "description": "List targets configured in leedevkit.toml", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
	}
}

func mcpToolResult(value any, isError bool) map[string]any {
	data, _ := json.Marshal(value)
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(data)}}, "isError": isError}
}

func callProjectMCPTool(raw json.RawMessage, root string, diagnostics io.Writer) (any, map[string]any) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || params.Name == "" {
		return nil, map[string]any{"code": -32602, "message": "tool name is required"}
	}
	switch params.Name {
	case "run_test":
		var args struct {
			Target string   `json:"target"`
			Flags  []string `json:"flags"`
		}
		if err := decodeMCPArguments(params.Arguments, &args); err != nil || strings.TrimSpace(args.Target) == "" || strings.HasPrefix(args.Target, "-") {
			return nil, map[string]any{"code": -32602, "message": "target and valid flags are required"}
		}
		allowed := map[string]bool{"--lint-only": true, "--unit-only": true, "--e2e-only": true, "--coverage": true, "--skip-lint": true, "--skip-e2e": true, "--quiet": true, "--failed-only": true, "--compact": true}
		seen := make(map[string]bool, len(args.Flags))
		for _, flag := range args.Flags {
			if !allowed[flag] || seen[flag] {
				return nil, map[string]any{"code": -32602, "message": fmt.Sprintf("unsupported or repeated test flag %q", flag)}
			}
			seen[flag] = true
		}
		project, err := config.Load(filepath.Join(root, "leedevkit.toml"))
		if err != nil {
			return mcpToolResult(map[string]any{"error": "unable to load project configuration"}, true), nil
		}
		if args.Target != "all" && args.Target != "infra" {
			if _, ok := project.Targets[args.Target]; !ok {
				if _, ok := project.Services[args.Target]; !ok {
					return nil, map[string]any{"code": -32602, "message": "target is not configured"}
				}
			}
		}
		var stdout, stderr bytes.Buffer
		cliArgs := append([]string{"test", args.Target}, args.Flags...)
		cliArgs = append(cliArgs, "--json", "--quiet")
		code := run(cliArgs, strings.NewReader(""), &stdout, &stderr)
		if stderr.Len() > 0 {
			_, _ = io.WriteString(diagnostics, process.RedactDiagnostic(stderr.String()))
		}
		text := strings.TrimSpace(stdout.String())
		if code != 0 && text == "" {
			text = process.RedactDiagnostic(stderr.String())
		}
		return mcpToolResult(map[string]any{"exitCode": code, "output": text}, code != 0), nil
	case "get_project_status":
		if err := decodeMCPArguments(params.Arguments, &struct{}{}); err != nil {
			return nil, map[string]any{"code": -32602, "message": "arguments must be an empty object"}
		}
		var stdout, stderr bytes.Buffer
		result := process.Run(context.Background(), process.Request{Args: []string{"git", "status", "--short", "--branch"}, Dir: root, Stdout: &stdout, Stderr: &stderr})
		status := "unavailable"
		if result.ExitCode == 0 {
			status = strings.TrimSpace(stdout.String())
		}
		return mcpToolResult(map[string]any{"root": root, "manifest": fileExists(filepath.Join(root, "leedevkit.toml")), "git": status}, result.ExitCode != 0), nil
	case "list_targets":
		if err := decodeMCPArguments(params.Arguments, &struct{}{}); err != nil {
			return nil, map[string]any{"code": -32602, "message": "arguments must be an empty object"}
		}
		project, err := config.Load(filepath.Join(root, "leedevkit.toml"))
		if err != nil {
			return mcpToolResult(map[string]any{"error": "unable to load project configuration"}, true), nil
		}
		return mcpToolResult(describeTargets(project), false), nil
	default:
		return nil, map[string]any{"code": -32601, "message": "Unknown tool"}
	}
}

func decodeMCPArguments(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("unexpected trailing arguments")
	}
	return nil
}
