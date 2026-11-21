package debug

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Runtime struct{ Environment Environment }
type Logs struct {
	Status  string
	Service string
	Source  string
	Lines   []string
}
type Schema struct {
	Status       string
	Schemas      []string
	MetadataOnly bool
}
type HTTPResult struct {
	Status     string
	HTTPStatus int
	Body       string
}

func NewRuntime(environment Environment) Runtime { return Runtime{Environment: environment} }
func (r Runtime) Metadata() map[string]any       { return r.Environment.Metadata() }
func (r Runtime) service(id string) (Service, error) {
	service, ok := r.Environment.Services[id]
	if !ok {
		return Service{}, ManifestError{"unknown approved service"}
	}
	return service, nil
}
func (r Runtime) Health(id, path string) (HTTPResult, error) {
	service, err := r.service(id)
	if err != nil {
		return HTTPResult{}, err
	}
	if !contains(service.Health, path) {
		return HTTPResult{}, ManifestError{"health path is not approved"}
	}
	return r.get(service, path)
}
func (r Runtime) HTTPGet(id, path string) (HTTPResult, error) {
	service, err := r.service(id)
	if err != nil {
		return HTTPResult{}, err
	}
	if !contains(service.HTTPGet, path) {
		return HTTPResult{}, ManifestError{"GET path is not approved"}
	}
	return r.get(service, path)
}
func (r Runtime) get(service Service, path string) (HTTPResult, error) {
	if _, err := url.Parse(path); err != nil {
		return HTTPResult{}, err
	}
	response, err := http.Get(fmt.Sprintf("http://%s:%d%s", service.Host, service.Port, path))
	if err != nil {
		return HTTPResult{Status: "unavailable"}, nil
	}
	defer response.Body.Close()
	buffer := make([]byte, r.Environment.Limits["max_bytes"]+1)
	count, _ := response.Body.Read(buffer)
	return HTTPResult{Status: "ok", HTTPStatus: response.StatusCode, Body: redactText(string(buffer[:min(count, r.Environment.Limits["max_bytes"])]), r.Environment.Redaction)}, nil
}
func (r Runtime) RecentLogs(id, source, severity string) (Logs, error) {
	service, err := r.service(id)
	if err != nil {
		return Logs{}, err
	}
	if !contains(service.LogFiles, source) {
		return Logs{}, ManifestError{"log source is not approved"}
	}
	root, _ := filepath.Abs(r.Environment.ProjectRoot)
	path := filepath.Join(root, source)
	relative, _ := filepath.Rel(root, path)
	if !relativeSafe(relative) {
		return Logs{}, ManifestError{"log source must stay inside project root"}
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return Logs{}, ManifestError{"approved log source unavailable"}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Logs{}, err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	max := r.Environment.Limits["max_log_lines"]
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	filtered := []string{}
	for _, line := range lines {
		if severity == "" || strings.Contains(strings.ToLower(line), strings.ToLower(severity)) {
			filtered = append(filtered, redactText(line, r.Environment.Redaction))
		}
	}
	return Logs{Status: "ok", Service: id, Source: source, Lines: filtered}, nil
}
func (r Runtime) DatabaseSchema() (Schema, error) {
	if r.Environment.Database["enabled"] != true {
		return Schema{}, ManifestError{"database capability unavailable"}
	}
	schemas, _ := r.Environment.Database["schemas"].([]any)
	result := make([]string, len(schemas))
	for i, v := range schemas {
		result[i], _ = v.(string)
	}
	return Schema{Status: "ok", Schemas: result, MetadataOnly: true}, nil
}
func (r Runtime) DatabaseQuery(template string, parameters map[string]any) (map[string]any, error) {
	templates, _ := r.Environment.Database["templates"].(map[string]any)
	query, ok := templates[template]
	if !ok {
		return nil, ManifestError{"database query template is not approved"}
	}
	text, ok := query.(string)
	if !ok || !readOnlySQL(text) {
		return nil, ManifestError{"database query template is not read-only"}
	}
	return nil, ManifestError{"database adapter not configured"}
}
func readOnlySQL(query string) bool {
	upper := strings.ToUpper(query)
	for _, word := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", ";"} {
		if strings.Contains(upper, word) {
			return false
		}
	}
	return true
}
func redactText(value string, patterns []string) string {
	result := value
	for _, pattern := range patterns {
		if expression, err := regexp.Compile(pattern); err == nil {
			result = expression.ReplaceAllString(result, "[REDACTED]")
		}
	}
	for _, key := range []string{"password", "secret", "token", "authorization", "cookie", "api_key", "jwt"} {
		result = regexp.MustCompile(`(?i)`+key+`\s*[:=]\s*[^\s,;]+`).ReplaceAllString(result, key+"=[REDACTED]")
	}
	return result
}
func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = json.Valid
