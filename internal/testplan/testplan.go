package testplan

import (
	"fmt"
	"sort"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/modes"
)

type Alias struct {
	Name     string
	Services []string
}

type Service struct {
	Name     string
	Language string
}

type Request struct {
	Target      string
	AllServices []string
	Aliases     []Alias
	Services    []Service
}

func RequestFromConfig(target string, project config.Project) Request {
	services := make([]Service, 0, len(project.Services))
	serviceNames := make([]string, 0, len(project.Services))
	for name := range project.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)
	for _, name := range serviceNames {
		services = append(services, Service{Name: name, Language: project.Services[name].Language})
	}
	allServices := project.Targets["all"]
	aliases := make([]Alias, 0, len(project.Targets))
	targetNames := make([]string, 0, len(project.Targets))
	for name := range project.Targets {
		if name != "all" {
			targetNames = append(targetNames, name)
		}
	}
	sort.Strings(targetNames)
	for _, name := range targetNames {
		aliases = append(aliases, Alias{Name: name, Services: project.Targets[name]})
	}
	return Request{Target: target, AllServices: allServices, Aliases: aliases, Services: services}
}

type Result struct {
	Targets []string
	Modes   map[string]string
}

func Plan(request Request) (Result, error) {
	if request.Target == "" {
		return Result{}, fmt.Errorf("test target is required")
	}
	resolvedTargets, err := ExpandTargets(request)
	if err != nil {
		return Result{}, err
	}
	serviceDefinitions := make([]modes.Service, len(request.Services))
	for index, service := range request.Services {
		serviceDefinitions[index] = modes.Service{Name: service.Name, Language: service.Language}
	}
	return Result{
		Targets: resolvedTargets,
		Modes:   modes.Resolve(serviceDefinitions),
	}, nil
}
