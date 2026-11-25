package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/vkaylee/leedevkit/internal/gates"
)

var namespacePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)

type Service struct {
	Language    string
	Workdir     string
	Name        string
	Dockerfile  string
	Ports       []int
	HealthCheck string
	Go          bool
	DevReload   string
}

type DevMigrate struct {
	Service       string   `toml:"service"`
	Command       []string `toml:"command"`
	StatusCommand []string `toml:"status_command"`
	Dir           string   `toml:"dir"`
	DBService     string   `toml:"db_service"`
	DBUser        string   `toml:"db_user"`
	DBName        string   `toml:"db_name"`
}

func (p Project) ServiceForLanguage(language string) (Service, bool) {
	names := make([]string, 0, len(p.Services))
	for name, service := range p.Services {
		if service.Language == language {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return Service{}, false
	}
	sort.Strings(names)
	for _, name := range names {
		if name == language {
			service := p.Services[name]
			service.Name = name
			return service, true
		}
	}
	name := names[0]
	service := p.Services[name]
	service.Name = name
	return service, true
}

// GatesForService returns gates matching phase, preferring target/service-scoped overrides over global defaults.
func (p Project) GatesForService(target, phase string) []gates.Gate {
	matchNames := map[string]bool{target: true}
	for alias, services := range p.Targets {
		if alias == target {
			for _, svc := range services {
				matchNames[svc] = true
			}
		} else if len(services) == 1 && services[0] == target {
			matchNames[alias] = true
		}
	}
	var scoped, global []gates.Gate
	for _, gate := range p.Gates {
		if gate.Phase != phase {
			continue
		}
		if gate.Target != "" {
			if matchNames[gate.Target] {
				scoped = append(scoped, gate)
			}
			continue
		}
		global = append(global, gate)
	}
	if len(scoped) > 0 {
		return scoped
	}
	return global
}

type Project struct {
	Name           string
	Namespace      string
	Languages      []string
	Services       map[string]Service
	Targets        map[string][]string
	Dependencies   map[string][]string
	BuildPolicy    string
	Gates          []gates.Gate
	DefaultGoGates bool
	AutoMigrate    bool
	DevMigrate     DevMigrate
	Harnesses      []string
}

type rawTestGate struct {
	Phase           string   `toml:"phase"`
	Command         []string `toml:"command"`
	Mode            string   `toml:"mode"`
	WorkingDir      string   `toml:"working_dir"`
	Service         string   `toml:"service"`
	Timeout         *float64 `toml:"timeout"`
	Required        *bool    `toml:"required"`
	Condition       string   `toml:"condition"`
	FailurePolicy   string   `toml:"failure_policy"`
	Environment     []string `toml:"environment"`
	Requires        []string `toml:"requires"`
	ExpectedVersion *string  `toml:"expected_version"`
	VersionCommand  []string `toml:"version_command"`
	FixCommand      []string `toml:"fix_command"`
	NewFindingsFlag []string `toml:"new_findings_flag"`
	SetupCommand    []string `toml:"setup_command"`
	TeardownCommand []string `toml:"teardown_command"`
}

func Load(path string) (Project, error) {
	project := Project{
		Services:       map[string]Service{},
		Targets:        map[string][]string{},
		Dependencies:   map[string][]string{},
		BuildPolicy:    "once",
		Gates:          gates.Defaults(),
		DefaultGoGates: true,
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return project, nil
		}
		return project, err
	}
	var raw struct {
		Languages []string `toml:"languages"`
		Project   struct {
			Name      string   `toml:"name"`
			Namespace string   `toml:"namespace"`
			Languages []string `toml:"languages"`
		} `toml:"project"`
		Services map[string]struct {
			Lang        string `toml:"lang"`
			Workdir     string `toml:"workdir"`
			Dockerfile  string `toml:"dockerfile"`
			Ports       []int  `toml:"ports"`
			HealthCheck string `toml:"health_check"`
			Go          bool   `toml:"go"`
			DevReload   string `toml:"dev_reload"`
			Test        struct {
				Lint        any `toml:"lint"`
				Unit        any `toml:"unit"`
				Integration any `toml:"integration"`
				Coverage    any `toml:"coverage"`
			} `toml:"test"`
		} `toml:"services"`
		Targets map[string][]string `toml:"targets"`
		Test    struct {
			BuildPolicy  string                 `toml:"build_policy"`
			Dependencies map[string][]string    `toml:"dependencies"`
			Gates        map[string]rawTestGate `toml:"gates"`
		} `toml:"test"`
		Dev struct {
			AutoMigrate bool       `toml:"auto_migrate"`
			Migrate     DevMigrate `toml:"migrate"`
		} `toml:"dev"`
		AI struct {
			Harnesses []string `toml:"harnesses"`
		} `toml:"ai"`
		Quality struct {
			Gates map[string]gates.Gate `toml:"gates"`
			Go    struct {
				Lint        []string `toml:"lint"`
				Unit        []string `toml:"unit"`
				Integration []string `toml:"integration"`
				Generated   []string `toml:"generated"`
			} `toml:"go"`
		} `toml:"quality"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return project, err
	}
	project.Name = strings.TrimSpace(raw.Project.Name)
	project.Namespace = strings.TrimSpace(raw.Project.Namespace)
	if project.Namespace != "" && (strings.HasPrefix(project.Namespace, "pod_") || !namespacePattern.MatchString(project.Namespace)) {
		return project, fmt.Errorf("project.namespace %q must match %s", project.Namespace, namespacePattern.String())
	}
	project.AutoMigrate = raw.Dev.AutoMigrate
	project.DevMigrate = raw.Dev.Migrate
	project.Harnesses = append([]string(nil), raw.AI.Harnesses...)
	serviceNames := make([]string, 0, len(raw.Services))
	for name := range raw.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)
	for _, name := range serviceNames {
		service := raw.Services[name]
		lang := service.Lang
		if lang == "" && service.Go {
			lang = "go"
		}
		project.Services[name] = Service{
			Name:        name,
			Language:    lang,
			Workdir:     service.Workdir,
			Dockerfile:  service.Dockerfile,
			Ports:       append([]int(nil), service.Ports...),
			HealthCheck: service.HealthCheck,
			Go:          service.Go,
			DevReload:   service.DevReload,
		}
	}
	project.Languages = append([]string(nil), raw.Project.Languages...)
	if len(project.Languages) == 0 {
		project.Languages = append([]string(nil), raw.Languages...)
	}
	if raw.Targets != nil {
		project.Targets = raw.Targets
	}
	if raw.Test.Dependencies != nil {
		for key, deps := range raw.Test.Dependencies {
			if len(deps) == 0 {
				return project, fmt.Errorf("test.dependencies.%s must list at least one compose service", key)
			}
			for _, dep := range deps {
				if strings.TrimSpace(dep) == "" {
					return project, fmt.Errorf("test.dependencies.%s must not contain empty service names", key)
				}
			}
		}
		project.Dependencies = raw.Test.Dependencies
	}
	selected := append([]string{}, raw.Quality.Go.Lint...)
	selected = append(selected, raw.Quality.Go.Unit...)
	selected = append(selected, raw.Quality.Go.Integration...)
	selected = append(selected, raw.Quality.Go.Generated...)
	if len(selected) > 0 {
		project.DefaultGoGates = false
		project.Gates = make([]gates.Gate, 0, len(selected))
		seen := map[string]bool{}
		for _, name := range selected {
			if seen[name] {
				continue
			}
			gate, ok := raw.Quality.Gates[name]
			if !ok {
				return project, fmt.Errorf("quality gate %q is not defined", name)
			}
			gate.Name = name
			if gate.Mode == "" {
				gate.Mode = "run"
			}
			if gate.Condition == "" {
				gate.Condition = "always"
			}
			if gate.FailurePolicy == "" {
				gate.FailurePolicy = "required"
				if !gate.Required {
					gate.FailurePolicy = "optional"
				}
			}
			if gate.Environment == nil {
				gate.Environment = []string{}
			}
			if gate.Requires == nil {
				gate.Requires = []string{}
			}
			if err := gates.Validate(gate); err != nil {
				return project, fmt.Errorf("quality gate %q is invalid: %w", name, err)
			}
			seen[name] = true
			project.Gates = append(project.Gates, gate)
		}
	}

	// Explicit [services.<name>.test] phase command overrides.
	for _, svcName := range serviceNames {
		svc := raw.Services[svcName]
		for _, spec := range []struct {
			phase string
			raw   any
		}{
			{"lint", svc.Test.Lint},
			{"unit", svc.Test.Unit},
			{"integration", svc.Test.Integration},
			{"coverage", svc.Test.Coverage},
		} {
			commands, err := parseServiceTestCommands(spec.raw)
			if err != nil {
				return project, fmt.Errorf("services.%s.test.%s: %w", svcName, spec.phase, err)
			}
			for idx, cmd := range commands {
				gateName := fmt.Sprintf("service-%s-%s", svcName, spec.phase)
				if len(commands) > 1 {
					gateName = fmt.Sprintf("%s-%d", gateName, idx+1)
				}
				gate := gates.Gate{
					Name:          gateName,
					Phase:         spec.phase,
					Target:        svcName,
					Service:       svcName,
					Command:       cmd,
					Mode:          "run",
					Required:      true,
					Condition:     "always",
					FailurePolicy: "required",
					Environment:   []string{},
					Requires:      []string{},
				}
				if err := gates.Validate(gate); err != nil {
					return project, fmt.Errorf("services.%s.test.%s is invalid: %w", svcName, spec.phase, err)
				}
				project.Gates = append(project.Gates, gate)
			}
		}
	}

	// Explicit [test.gates.<key>] phase command overrides.
	testGateKeys := make([]string, 0, len(raw.Test.Gates))
	for key := range raw.Test.Gates {
		testGateKeys = append(testGateKeys, key)
	}
	sort.Strings(testGateKeys)
	for _, key := range testGateKeys {
		rawGate := raw.Test.Gates[key]
		phase, targetHint, err := inferTestGatePhaseAndTarget(key, rawGate.Phase)
		if err != nil {
			return project, err
		}
		required := true
		if rawGate.Required != nil {
			required = *rawGate.Required
		}
		failurePolicy := rawGate.FailurePolicy
		if failurePolicy == "" {
			if required {
				failurePolicy = "required"
			} else {
				failurePolicy = "optional"
			}
		}
		mode := rawGate.Mode
		if mode == "" {
			mode = "run"
		}
		condition := rawGate.Condition
		if condition == "" {
			condition = "always"
		}
		env := rawGate.Environment
		if env == nil {
			env = []string{}
		}
		reqs := rawGate.Requires
		if reqs == nil {
			reqs = []string{}
		}
		gate := gates.Gate{
			Name:            key,
			Phase:           phase,
			Target:          targetHint,
			Command:         append([]string(nil), rawGate.Command...),
			Mode:            mode,
			WorkingDir:      rawGate.WorkingDir,
			Service:         rawGate.Service,
			Timeout:         rawGate.Timeout,
			Required:        required,
			Condition:       condition,
			FailurePolicy:   failurePolicy,
			Environment:     env,
			Requires:        reqs,
			ExpectedVersion: rawGate.ExpectedVersion,
			VersionCommand:  rawGate.VersionCommand,
			FixCommand:      rawGate.FixCommand,
			NewFindingsFlag: rawGate.NewFindingsFlag,
			SetupCommand:    rawGate.SetupCommand,
			TeardownCommand: rawGate.TeardownCommand,
		}
		if err := gates.Validate(gate); err != nil {
			return project, fmt.Errorf("test gate %q is invalid: %w", key, err)
		}
		if gate.Target == "" {
			project.Gates = removeGlobalPhaseGates(project.Gates, gate.Phase)
		}
		project.Gates = append(project.Gates, gate)
	}

	// Automatically enable default Go integration gate when default Go gates are active
	// and [test.dependencies] declares a Go integration phase without an existing global integration gate.
	if project.DefaultGoGates && project.hasGoIntegrationDependency() && !hasGlobalPhaseGate(project.Gates, "integration") {
		project.Gates = append(project.Gates, gates.DefaultIntegration())
	}

	if raw.Test.BuildPolicy != "" {
		if raw.Test.BuildPolicy != "once" && raw.Test.BuildPolicy != "phase" {
			return project, errors.New("test.build_policy must be once or phase")
		}
		project.BuildPolicy = raw.Test.BuildPolicy
	}
	return project, nil
}

func (p Project) hasGoIntegrationDependency() bool {
	for key := range p.Dependencies {
		if !strings.HasPrefix(key, "int-") {
			continue
		}
		suffix := strings.TrimPrefix(key, "int-")
		if suffix == "go" {
			return true
		}
		if svc, ok := p.Services[suffix]; ok && svc.Language == "go" {
			return true
		}
		for _, svcName := range p.Targets[suffix] {
			if svc, ok := p.Services[svcName]; ok && svc.Language == "go" {
				return true
			}
		}
		for _, dep := range p.Dependencies[key] {
			if svc, ok := p.Services[dep]; ok && svc.Language == "go" {
				return true
			}
		}
	}
	return false
}

func hasGlobalPhaseGate(definitions []gates.Gate, phase string) bool {
	for _, gate := range definitions {
		if gate.Phase == phase && gate.Target == "" {
			return true
		}
	}
	return false
}

func removeGlobalPhaseGates(definitions []gates.Gate, phase string) []gates.Gate {
	filtered := make([]gates.Gate, 0, len(definitions))
	for _, gate := range definitions {
		if gate.Phase == phase && gate.Target == "" {
			continue
		}
		filtered = append(filtered, gate)
	}
	return filtered
}

func inferTestGatePhaseAndTarget(key, explicitPhase string) (string, string, error) {
	for _, entry := range []struct {
		exact  string
		prefix string
		phase  string
	}{
		{"integration", "int-", "integration"},
		{"unit", "unit-", "unit"},
		{"lint", "lint-", "lint"},
		{"coverage", "cov-", "coverage"},
		{"generated", "gen-", "generated"},
	} {
		if key == entry.exact {
			phase := explicitPhase
			if phase == "" {
				phase = entry.phase
			}
			return phase, "", nil
		}
		if strings.HasPrefix(key, entry.prefix) {
			phase := explicitPhase
			if phase == "" {
				phase = entry.phase
			}
			suffix := strings.TrimPrefix(key, entry.prefix)
			if suffix == "go" {
				suffix = ""
			}
			return phase, suffix, nil
		}
	}
	if explicitPhase != "" {
		return explicitPhase, "", nil
	}
	return "", "", fmt.Errorf("test gate %q must specify phase or use lint-/unit-/int- prefix", key)
}

func parseServiceTestCommands(raw any) ([][]string, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil, errors.New("must be a non-empty command array")
	}
	if _, isNested := items[0].([]any); isNested {
		commands := make([][]string, 0, len(items))
		for _, item := range items {
			nested, ok := item.([]any)
			if !ok || len(nested) == 0 {
				return nil, errors.New("each command must be a non-empty string array")
			}
			cmd, err := toStringCommand(nested)
			if err != nil {
				return nil, err
			}
			commands = append(commands, cmd)
		}
		return commands, nil
	}
	cmd, err := toStringCommand(items)
	if err != nil {
		return nil, err
	}
	return [][]string{cmd}, nil
}

func toStringCommand(items []any) ([]string, error) {
	cmd := make([]string, 0, len(items))
	for _, item := range items {
		str, ok := item.(string)
		if !ok || str == "" {
			return nil, errors.New("command arguments must be non-empty strings")
		}
		cmd = append(cmd, str)
	}
	return cmd, nil
}
