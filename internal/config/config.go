package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
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
	for name, service := range p.Services {
		if service.Language == language {
			service.Name = name
			return service, true
		}
	}
	return Service{}, false
}

type Project struct {
	Services     map[string]Service
	Targets      map[string][]string
	Dependencies map[string][]string
	BuildPolicy  string
}

func Load(path string) (Project, error) {
	project := Project{Services: map[string]Service{}, Targets: map[string][]string{}, Dependencies: map[string][]string{}, BuildPolicy: "once"}
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
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return project, err
	}
	for name, service := range raw.Services {
		project.Services[name] = Service{Name: name, Language: service.Lang, Workdir: service.Workdir, Image: service.Image, Dockerfile: service.Dockerfile, ComposeFile: service.ComposeFile}
	}
	project.Targets = raw.Targets
	project.Dependencies = raw.Test.Dependencies
	if raw.Test.BuildPolicy != "" {
		if raw.Test.BuildPolicy != "once" && raw.Test.BuildPolicy != "phase" {
			return project, errors.New("test.build_policy must be once or phase")
		}
		project.BuildPolicy = raw.Test.BuildPolicy
	}
	return project, nil
}
