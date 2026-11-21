package debug

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type ManifestError struct{ Message string }

func (e ManifestError) Error() string { return e.Message }

type Service struct {
	ID       string
	Host     string
	Port     int
	Aliases  []string
	Health   []string
	HTTPGet  []string
	LogFiles []string
}

type Environment struct {
	Name           string
	ComposeProject string
	ComposeFiles   []string
	Profiles       []string
	Networks       []string
	Services       map[string]Service
	Database       map[string]any
	Redaction      []string
	SensitiveCols  []string
	Limits         map[string]int
	ProjectRoot    string
	SourcePath     string
}

type Options struct{ ProjectRoot string }

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)

func Load(path, name string, options Options) (Environment, error) {
	var result Environment
	if !safeID.MatchString(name) || strings.Contains(strings.ToLower(name), "prod") {
		return result, ManifestError{"production environments are not eligible for Debug MCP"}
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return result, ManifestError{fmt.Sprintf("manifest not found: %s", path)}
	}
	var raw struct {
		Debug struct {
			ManifestVersion int `toml:"manifest_version"`
			Environments    map[string]struct {
				ComposeProject   string   `toml:"compose_project"`
				ComposeFiles     []string `toml:"compose_files"`
				Profiles         []string `toml:"profiles"`
				ApprovedNetworks []string `toml:"approved_networks"`
				Services         map[string]struct {
					Aliases      []string `toml:"aliases"`
					InternalHost string   `toml:"internal_host"`
					InternalPort int      `toml:"internal_port"`
					HealthPaths  []string `toml:"health_paths"`
					HTTPGetPaths []string `toml:"http_get_paths"`
					LogSources   []string `toml:"log_sources"`
				} `toml:"services"`
				Database  map[string]any `toml:"database"`
				Redaction struct {
					Patterns         []string `toml:"patterns"`
					SensitiveColumns []string `toml:"sensitive_columns"`
				} `toml:"redaction"`
			} `toml:"environments"`
		} `toml:"debug"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil || raw.Debug.ManifestVersion != 1 {
		return result, ManifestError{"manifest is not valid Debug MCP TOML"}
	}
	cfg, ok := raw.Debug.Environments[name]
	if !ok || !safeID.MatchString(cfg.ComposeProject) || strings.Contains(strings.ToLower(cfg.ComposeProject), "prod") {
		return result, ManifestError{"unknown or unsafe debug environment"}
	}
	root, err := filepath.Abs(options.ProjectRoot)
	if err != nil {
		return result, err
	}
	for _, file := range cfg.ComposeFiles {
		if !relativeSafe(file) || !fileExists(filepath.Join(root, file)) {
			return result, ManifestError{"compose file must stay inside project root"}
		}
	}
	for _, network := range cfg.ApprovedNetworks {
		if !safeID.MatchString(network) {
			return result, ManifestError{"network must be a safe identifier"}
		}
	}
	services := map[string]Service{}
	for id, service := range cfg.Services {
		if !safeID.MatchString(id) || !safeHost(service.InternalHost) || service.InternalPort < 1 || service.InternalPort > 65535 {
			return result, ManifestError{"service is unsafe"}
		}
		for _, path := range append(append([]string{}, service.HealthPaths...), service.HTTPGetPaths...) {
			if !safeURLPath(path) {
				return result, ManifestError{"service path is unsafe"}
			}
		}
		for _, file := range service.LogSources {
			if !relativeSafe(file) {
				return result, ManifestError{"log source must be relative"}
			}
		}
		services[id] = Service{ID: id, Host: service.InternalHost, Port: service.InternalPort, Aliases: service.Aliases, Health: service.HealthPaths, HTTPGet: service.HTTPGetPaths, LogFiles: service.LogSources}
	}
	for _, pattern := range cfg.Redaction.Patterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return result, ManifestError{"redaction pattern is invalid"}
		}
	}
	return Environment{Name: name, ComposeProject: cfg.ComposeProject, ComposeFiles: cfg.ComposeFiles, Profiles: cfg.Profiles, Networks: cfg.ApprovedNetworks, Services: services, Database: cfg.Database, Redaction: cfg.Redaction.Patterns, SensitiveCols: cfg.Redaction.SensitiveColumns, Limits: map[string]int{"request_timeout_ms": 5000, "max_bytes": 65536, "max_log_lines": 200}, ProjectRoot: root, SourcePath: path}, nil
}

func (e Environment) Metadata() map[string]any {
	names := make([]string, 0, len(e.Services))
	for name := range e.Services {
		names = append(names, name)
	}
	return map[string]any{"environment": e.Name, "compose_project": e.ComposeProject, "mode": "read-only", "services": names, "networks": e.Networks, "limits": e.Limits}
}
func relativeSafe(path string) bool {
	clean := filepath.Clean(path)
	return path != "" && !filepath.IsAbs(path) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
func safeURLPath(path string) bool {
	parsed, err := url.Parse(path)
	return err == nil && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && parsed.RawQuery == "" && parsed.Fragment == ""
}
func safeHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") || strings.EqualFold(host, "host.docker.internal") || strings.EqualFold(host, "host.containers.internal") {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || (!ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !ip.IsPrivate())
}
