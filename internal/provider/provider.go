package provider

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
	host, err := load(options.Host)
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
		selected, source = hostName, options.Host
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

	raw := map[string]string{}
	for key, value := range repo["container.providers."+selected] {
		raw[key] = value
	}
	for key, value := range host["container.providers."+selected] {
		raw[key] = value
	}
	if len(raw) == 0 && selected == "docker" {
		raw["cli"] = `['docker']`
		raw["compose"] = `['docker', 'compose']`
		source = "LeeDevKit default"
	}
	cli, err := parseStringArray(raw["cli"])
	if err != nil {
		return Provider{}, fmt.Errorf("provider %q cli: %w", selected, err)
	}
	compose, err := parseStringArray(raw["compose"])
	if err != nil {
		return Provider{}, fmt.Errorf("provider %q compose: %w", selected, err)
	}
	capabilities, err := parseStringArray(raw["capabilities"])
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
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return nil, err
	}
	defer file.Close()
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if result[section] == nil {
				result[section] = map[string]string{}
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section == "" {
			continue
		}
		result[section][strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func stripComment(line string) string {
	quoted := false
	for index, character := range line {
		if character == '\'' || character == '"' {
			quoted = !quoted
		}
		if character == '#' && !quoted {
			return line[:index]
		}
	}
	return line
}

func parseStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("must be a non-empty string array")
	}
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil, errors.New("must be a string array")
	}
	body := strings.TrimSpace(value[1 : len(value)-1])
	if body == "" {
		return nil, errors.New("must be a non-empty string array")
	}
	var result []string
	for _, item := range strings.Split(body, ",") {
		item = strings.TrimSpace(item)
		if len(item) < 2 || (item[0] != '\'' && item[0] != '"') || item[len(item)-1] != item[0] {
			return nil, errors.New("must contain only quoted strings")
		}
		item = strings.TrimSpace(item[1 : len(item)-1])
		if item == "" {
			return nil, errors.New("must contain only non-empty strings")
		}
		result = append(result, item)
	}
	return result, nil
}
