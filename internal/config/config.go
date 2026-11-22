package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
	"github.com/vkaylee/leedevkit/internal/gates"
)

type Service struct {
	Language    string
	Workdir     string
	Name        string
	Image       string
	Dockerfile  string
	ComposeFile string
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

type Project struct {
	Services     map[string]Service
	Targets      map[string][]string
	Dependencies map[string][]string
	BuildPolicy  string
	Gates        []gates.Gate
}

func Load(path string) (Project, error) {
	project := Project{Services: map[string]Service{}, Targets: map[string][]string{}, Dependencies: map[string][]string{}, BuildPolicy: "once", Gates: gates.Defaults()}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return project, nil
		}
		return project, err
	}
	var raw struct {
		Services map[string]struct {
			Lang        string `toml:"lang"`
			Workdir     string `toml:"workdir"`
			Image       string `toml:"image"`
			Dockerfile  string `toml:"dockerfile"`
			ComposeFile string `toml:"compose_file"`
		} `toml:"services"`
		Targets map[string][]string `toml:"targets"`
		Test    struct {
			BuildPolicy  string              `toml:"build_policy"`
			Dependencies map[string][]string `toml:"dependencies"`
		} `toml:"test"`
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
	for name, service := range raw.Services {
		project.Services[name] = Service{Name: name, Language: service.Lang, Workdir: service.Workdir, Image: service.Image, Dockerfile: service.Dockerfile, ComposeFile: service.ComposeFile}
	}
	project.Targets = raw.Targets
	project.Dependencies = raw.Test.Dependencies
	selected := append([]string{}, raw.Quality.Go.Lint...)
	selected = append(selected, raw.Quality.Go.Unit...)
	selected = append(selected, raw.Quality.Go.Integration...)
	selected = append(selected, raw.Quality.Go.Generated...)
	if len(selected) > 0 {
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
	if raw.Test.BuildPolicy != "" {
		if raw.Test.BuildPolicy != "once" && raw.Test.BuildPolicy != "phase" {
			return project, errors.New("test.build_policy must be once or phase")
		}
		project.BuildPolicy = raw.Test.BuildPolicy
	}
	return project, nil
}
