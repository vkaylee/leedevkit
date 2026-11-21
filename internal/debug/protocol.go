package debug

import (
	"encoding/json"
	"fmt"
)

const (
	MaxProtocolString = 4096
	MaxRequestBytes   = 256 * 1024
	protocolVersion   = "2024-11-05"
)

type Server struct{ Runtime Runtime }

func NewServer(environment Environment) Server { return Server{Runtime: NewRuntime(environment)} }

func EncodeRequest(value map[string]any) ([]byte, error) { return json.Marshal(value) }

func DecodeRequest(data []byte) (map[string]any, error) {
	if len(data) > MaxRequestBytes {
		return nil, fmt.Errorf("request exceeds size limit")
	}
	var request map[string]any
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, err
	}
	return request, nil
}

func (s Server) Handle(request map[string]any) map[string]any {
	id := request["id"]
	if request["jsonrpc"] != "2.0" {
		return errorResponse(id, -32600, "Invalid Request")
	}
	method, ok := request["method"].(string)
	if !ok {
		return errorResponse(id, -32600, "Invalid Request")
	}
	if _, present := request["id"]; !present {
		return nil
	}
	switch method {
	case "initialize":
		return response(id, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{}, "resources": map[string]any{}, "prompts": map[string]any{},
			},
			"serverInfo": map[string]any{"name": "leedevkit-debug", "version": "1.0"},
		})
	case "ping":
		return response(id, map[string]any{})
	case "tools/list":
		return response(id, map[string]any{"tools": toolDefinitions()})
	case "resources/list":
		return response(id, map[string]any{"resources": resourceDefinitions()})
	case "prompts/list":
		return response(id, map[string]any{"prompts": promptDefinitions()})
	case "tools/call":
		return s.callTool(id, request["params"])
	case "resources/read":
		return s.readResource(id, request["params"])
	case "prompts/get":
		return promptGet(id, request["params"])
	default:
		return errorResponse(id, -32601, "Method not found")
	}
}

func (s Server) callTool(id any, raw any) map[string]any {
	params, ok := raw.(map[string]any)
	if !ok {
		return errorResponse(id, -32602, "params must be an object")
	}
	name, ok := params["name"].(string)
	if !ok {
		return errorResponse(id, -32602, "tool name is required")
	}
	arguments, _ := params["arguments"].(map[string]any)
	for key, value := range arguments {
		if len(key) > 128 {
			return errorResponse(id, -32602, "invalid argument keys")
		}
		if text, ok := value.(string); ok && len(text) > MaxProtocolString {
			return errorResponse(id, -32602, "argument string exceeds limit")
		}
	}

	var value any
	var err error
	switch name {
	case "debug_environment":
		value = s.Runtime.Metadata()
	case "service_health":
		value, err = s.Runtime.Health(stringArg(arguments, "service"), stringArg(arguments, "path"))
	case "http_get":
		value, err = s.Runtime.HTTPGet(stringArg(arguments, "service"), stringArg(arguments, "path"))
	case "recent_logs":
		severity, _ := arguments["severity"].(string)
		value, err = s.Runtime.RecentLogs(stringArg(arguments, "service"), stringArg(arguments, "source"), severity)
	case "database_schema":
		value, err = s.Runtime.DatabaseSchema()
	case "database_query":
		template := stringArg(arguments, "template")
		parameters, _ := arguments["parameters"].(map[string]any)
		value, err = s.Runtime.DatabaseQuery(template, parameters)
	default:
		return errorResponse(id, -32601, "Unknown tool")
	}
	if err != nil {
		return errorResponse(id, -32602, err.Error())
	}
	return response(id, map[string]any{
		"content": []any{map[string]any{"type": "text", "text": string(mustJSON(RedactValue(value, s.Runtime.Environment.Redaction)))}},
		"isError": false,
	})
}

func (s Server) readResource(id any, raw any) map[string]any {
	params, _ := raw.(map[string]any)
	uri, _ := params["uri"].(string)
	var value any
	switch uri {
	case "debug://safety":
		value = "Read-only only. No writes, migrations, arbitrary URLs, credentials, or conclusions without evidence."
	case "debug://runbook":
		value = "Inspect environment, health, logs, approved HTTP evidence, and approved database evidence."
	case "debug://environment":
		value = s.Runtime.Metadata()
	case "debug://capabilities":
		value = s.Runtime.Metadata()["capabilities"]
	default:
		return errorResponse(id, -32601, "Unknown resource")
	}
	return response(id, map[string]any{"contents": []any{map[string]any{"uri": uri, "mimeType": "text/plain", "text": string(mustJSON(value))}}})
}

func promptGet(id any, raw any) map[string]any {
	params, _ := raw.(map[string]any)
	if params["name"] != "debug_incident" {
		return errorResponse(id, -32601, "Unknown prompt")
	}
	arguments, _ := params["arguments"].(map[string]any)
	symptom, ok := arguments["symptom"].(string)
	if !ok || symptom == "" || len(symptom) > MaxProtocolString {
		return errorResponse(id, -32602, "symptom is required and bounded")
	}
	return response(id, map[string]any{
		"description": "Generic incident debugging",
		"messages":    []any{map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": "Symptom: " + symptom}}},
	})
}

func stringArg(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func response(id, value any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": value}
}

func errorResponse(id any, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

func mustJSON(value any) []byte { data, _ := json.Marshal(value); return data }

func toolDefinitions() []map[string]any {
	return []map[string]any{{"name": "debug_environment"}, {"name": "service_health"}, {"name": "http_get"}, {"name": "recent_logs"}, {"name": "database_schema"}, {"name": "database_query"}}
}

func resourceDefinitions() []map[string]any {
	return []map[string]any{{"uri": "debug://runbook"}, {"uri": "debug://safety"}, {"uri": "debug://environment"}, {"uri": "debug://capabilities"}, {"uri": "debug://evidence"}}
}

func promptDefinitions() []map[string]any { return []map[string]any{{"name": "debug_incident"}} }
