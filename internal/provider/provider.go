package provider

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var allCapabilities = []string{
	"build", "compose", "config", "down", "exec", "healthcheck", "image", "inspect",
	"logs", "network", "project_cleanup", "ps", "pull", "rm", "run", "up", "volume", "wait",
}

var secretKey = regexp.MustCompile(`(?i)(password|passwd|token|secret|private[-_]?key|api[-_]?key)`)

type Provider struct {
	Name         string         `json:"name"`
	CLI          []string       `json:"cli"`
	Compose      []string       `json:"compose"`
	Capabilities []string       `json:"capabilities"`
	Settings     map[string]any `json:"settings"`
	Source       string         `json:"source"`
}

type Options struct {
	Repository  string
	Host        string
	CLI         string
	Environment map[string]string
}

// ProtocolProvider is the normalized provider contract exchanged with the Go core.
// Python remains responsible for TOML loading while Go validates policy and diagnostics.
type ProtocolProvider struct {
	Name         string         `json:"provider"`
	CLI          []string       `json:"cli"`
	Compose      []string       `json:"compose"`
	Capabilities []string       `json:"capabilities"`
	Settings     map[string]any `json:"settings"`
	Source       string         `json:"source"`
}

func ValidateProtocol(input ProtocolProvider) (map[string]any, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("provider name must be non-empty")
	}
	if err := validateArgv(input.CLI, "cli"); err != nil {
		return nil, err
	}
	if err := validateArgv(input.Compose, "compose"); err != nil {
		return nil, err
	}
	valid := make(map[string]bool, len(allCapabilities))
	for _, capability := range allCapabilities {
		valid[capability] = true
	}
	seen := map[string]bool{}
	capabilities := make([]string, 0, len(input.Capabilities))
	for _, capability := range input.Capabilities {
		if !valid[capability] {
			return nil, fmt.Errorf("provider %q has unknown capability %q", input.Name, capability)
		}
		if !seen[capability] {
			seen[capability] = true
			capabilities = append(capabilities, capability)
		}
	}
	sort.Strings(capabilities)
	settings := input.Settings
	if settings == nil {
		settings = map[string]any{}
	}
	return map[string]any{
		"provider":     input.Name,
		"cli":          append([]string(nil), input.CLI...),
		"compose":      append([]string(nil), input.Compose...),
		"capabilities": capabilities,
		"settings":     redactValues(settings, ""),
		"source":       input.Source,
	}, nil
}

func validateArgv(values []string, name string) error {
	if len(values) == 0 {
		return fmt.Errorf("provider %s must be a non-empty string array", name)
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("provider %s must contain only non-empty strings", name)
		}
	}
	return nil
}

func redactValues(value any, key string) any {
	if secretKey.MatchString(key) {
		return "<redacted>"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			result[childKey] = redactValues(childValue, childKey)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, childValue := range typed {
			result[index] = redactValues(childValue, key)
		}
		return result
	case string:
		parsed, err := url.Parse(typed)
		if err == nil && parsed.User != nil {
			parsed.User = nil
			return parsed.String()
		}
	}
	return value
}

func Resolve(options Options) (Provider, error) {
	repo, err := load(options.Repository)
	if err != nil {
		return Provider{}, err
	}
	hostPath := options.Host
	if hostPath == "" {
		hostPath = HostConfigPath(options.Environment, "")
	}
	host, err := load(hostPath)
	if err != nil {
		return Provider{}, err
	}
	repoName := strings.Trim(strings.TrimSpace(repo["container"]["provider"]), "\"'")
	if repoName == "" {
		repoName = "docker"
	}
	hostName := strings.Trim(strings.TrimSpace(host["container"]["provider"]), "\"'")
	selected, source := repoName, "repository configuration"
	if hostName != "" {
		selected, source = hostName, hostPath
	}
	if options.Environment != nil && options.Environment["CONTAINER_PROVIDER"] != "" {
		selected, source = options.Environment["CONTAINER_PROVIDER"], "CONTAINER_PROVIDER"
	}
	if options.CLI != "" {
		selected, source = options.CLI, "CLI override"
	}
	if strings.TrimSpace(selected) == "" {
		return Provider{}, errors.New("container provider must be a non-empty string")
	}

	raw := mergeSettings(repo["container.providers."+selected], host["container.providers."+selected])
	if len(raw) == 0 && selected == "docker" {
		raw["cli"] = `['docker']`
		raw["compose"] = `['docker', 'compose']`
		source = "LeeDevKit default"
	}
	cli, err := parseTOMLStringArray(raw["cli"])
	if err != nil {
		return Provider{}, fmt.Errorf("provider %q cli: %w", selected, err)
	}
	compose, err := parseTOMLStringArray(raw["compose"])
	if err != nil {
		return Provider{}, fmt.Errorf("provider %q compose: %w", selected, err)
	}
	capabilities, err := parseTOMLStringArray(raw["capabilities"])
	if err != nil {
		if raw["capabilities"] != "" {
			return Provider{}, fmt.Errorf("provider %q capabilities: %w", selected, err)
		}
		capabilities = append([]string(nil), allCapabilities...)
	}
	valid := make(map[string]bool, len(allCapabilities))
	for _, capability := range allCapabilities {
		valid[capability] = true
	}
	for _, capability := range capabilities {
		if !valid[capability] {
			return Provider{}, fmt.Errorf("provider %q has unknown capability %q", selected, capability)
		}
	}
	sort.Strings(capabilities)
	settings := map[string]any{}
	for key, value := range raw {
		if key != "cli" && key != "compose" && key != "capabilities" {
			settings[key] = value
		}
	}
	return Provider{Name: selected, CLI: cli, Compose: compose, Capabilities: capabilities, Settings: settings, Source: source}, nil
}

func Diagnostics(p Provider) map[string]any {
	settings := map[string]any{}
	for key, value := range p.Settings {
		if secretKey.MatchString(key) {
			settings[key] = "<redacted>"
			continue
		}
		text, ok := value.(string)
		if !ok {
			settings[key] = value
			continue
		}
		text = strings.Trim(strings.TrimSpace(text), "\"'")
		if parsed, err := url.Parse(text); err == nil && parsed.User != nil {
			parsed.User = nil
			settings[key] = parsed.String()
			continue
		}
		settings[key] = text
	}
	capabilities := append([]string(nil), p.Capabilities...)
	sort.Strings(capabilities)
	return map[string]any{
		"provider":     p.Name,
		"cli":          append([]string(nil), p.CLI...),
		"compose":      append([]string(nil), p.Compose...),
		"capabilities": capabilities,
		"settings":     settings,
		"source":       p.Source,
	}
}

func load(path string) (map[string]map[string]string, error) {
	result := map[string]map[string]string{}
	if path == "" {
		return result, nil
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return nil, err
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	flattenTOML(document, "", result)
	return result, nil
}

func flattenTOML(value map[string]any, prefix string, result map[string]map[string]string) {
	for key, item := range value {
		section := key
		if prefix != "" {
			section = prefix + "." + key
		}
		if nested, ok := item.(map[string]any); ok {
			flattenTOML(nested, section, result)
			continue
		}
		if result[prefix] == nil {
			result[prefix] = map[string]string{}
		}
		result[prefix][key] = tomlValue(item)
	}
}

func tomlValue(value any) string {
	switch typed := value.(type) {
	case []string:
		parts := make([]string, len(typed))
		for index, item := range typed {
			parts[index] = fmt.Sprintf("%q", item)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case []any:
		parts := make([]string, len(typed))
		for index, item := range typed {
			parts[index] = fmt.Sprintf("%q", fmt.Sprint(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		return fmt.Sprintf("%v", value)
	}
}

func parseTOMLStringArray(value string) ([]string, error) {
	var wrapper struct {
		Value []string `toml:"value"`
	}
	quoted := strings.TrimSpace(value)
	if !strings.HasPrefix(quoted, "[") {
		return nil, errors.New("must be a string array")
	}
	if err := toml.Unmarshal([]byte("value = "+quoted), &wrapper); err != nil {
		return nil, err
	}
	values := wrapper.Value
	if len(values) == 0 {
		return nil, errors.New("must be a non-empty string array")
	}
	for _, item := range values {
		if strings.TrimSpace(item) == "" {
			return nil, errors.New("must contain only non-empty strings")
		}
	}
	return values, nil
}
