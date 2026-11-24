package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/vkaylee/leedevkit/internal/config"
)

type configuredTarget struct {
	Targets   []string `json:"targets,omitempty"`
	Languages []string `json:"languages"`
	Services  []string `json:"services"`
	Phases    []string `json:"phases"`
}

type targetsResponse struct {
	Targets map[string]configuredTarget `json:"targets"`
}

var configuredPhaseNames = []struct {
	name   string
	prefix string
}{
	{name: "unit", prefix: "unit-"},
	{name: "integration", prefix: "int-"},
	{name: "e2e", prefix: "e2e-"},
	{name: "lint", prefix: "lint-"},
}

func describeTargets(project config.Project) targetsResponse {
	result := targetsResponse{Targets: make(map[string]configuredTarget, len(project.Targets))}
	allTargets := make(map[string][]string, len(project.Targets)+len(project.Services))
	for name, selected := range project.Targets {
		allTargets[name] = selected
	}
	if len(project.Targets) == 0 {
		for name := range project.Services {
			allTargets[name] = []string{name}
		}
	}
	for _, target := range sortedKeys(allTargets) {
		selected := allTargets[target]
		entry := configuredTarget{Targets: []string{}, Languages: []string{}, Services: []string{}, Phases: []string{}}
		infraTarget := target == "infra"
		if infraTarget {
			entry.Languages = append(entry.Languages, project.Languages...)
			entry.Phases = append(entry.Phases, "infrastructure")
		}
		seenServices := make(map[string]bool)
		for _, name := range selected {
			if _, isService := project.Services[name]; isService {
				if !seenServices[name] {
					entry.Services = append(entry.Services, name)
					seenServices[name] = true
				}
				continue
			}
			if _, isTarget := project.Targets[name]; isTarget && name != target {
				entry.Targets = append(entry.Targets, name)
				for _, service := range project.Targets[name] {
					if _, ok := project.Services[service]; ok && !seenServices[service] {
						entry.Services = append(entry.Services, service)
						seenServices[service] = true
					}
				}
			}
		}
		languageSet := make(map[string]bool)
		for _, service := range entry.Services {
			language := project.Services[service].Language
			if language != "" && !languageSet[language] {
				entry.Languages = append(entry.Languages, language)
				languageSet[language] = true
			}
		}
		if containsTarget(entry.Targets, "infra") {
			entry.Phases = append(entry.Phases, "infrastructure")
		}
		for _, phase := range configuredPhaseNames {
			if infraTarget {
				break
			}
			configured := false
			if phase.name == "unit" || phase.name == "integration" || phase.name == "e2e" || phase.name == "lint" {
				configured = configured || hasRunnerPhase(entry, phase.name)
			}
			if phase.prefix != "" {
				for key := range project.Dependencies {
					if len(key) <= len(phase.prefix) || key[:len(phase.prefix)] != phase.prefix {
						continue
					}
					phaseTarget := key[len(phase.prefix):]
					if phaseTarget == target || containsTarget(entry.Targets, phaseTarget) || containsTarget(entry.Services, phaseTarget) {
						configured = true
						break
					}
				}
			}
			if !configured && !infraTarget {
				for _, child := range entry.Targets {
					if containsTarget(describeTargetPhases(project, child), phase.name) {
						configured = true
						break
					}
				}
			}
			if configured {
				entry.Phases = append(entry.Phases, phase.name)
			}
		}
		if len(entry.Phases) == 0 {
			continue
		}
		result.Targets[target] = entry
	}
	return result
}

func hasRunnerPhase(target configuredTarget, phase string) bool {
	phaseName := map[string]string{"unit": "Unit Tests", "e2e": "Integration Tests", "lint": "Linting"}[phase]
	if phaseName == "" {
		return false
	}
	for _, language := range target.Languages {
		if language != "typescript" {
			continue
		}
		commands := webPhaseCommands(phaseName)
		if phase == "unit" {
			return len(commands) > 0 && commands[0][2] == "test"
		}
		if phase == "e2e" {
			return len(commands) > 0 && commands[0][2] == "test:e2e"
		}
		if phase == "lint" {
			return len(commands) > 0 && commands[0][2] == "lint"
		}
	}
	return false
}

func describeTargetPhases(project config.Project, target string) []string {
	phases := make([]string, 0, len(configuredPhaseNames))
	for _, phase := range configuredPhaseNames {
		for key := range project.Dependencies {
			if len(key) > len(phase.prefix) && key[:len(phase.prefix)] == phase.prefix && key[len(phase.prefix):] == target {
				phases = append(phases, phase.name)
				break
			}
		}
	}
	return phases
}

func containsTarget(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func printTargets(args []string, stdout, stderr io.Writer, project config.Project) int {
	asJSON := false
	if len(args) > 1 {
		if len(args) != 2 || args[1] != "--json" {
			fmt.Fprintln(stderr, "usage: leedevkit targets [--json]")
			return 2
		}
		asJSON = true
	}
	result := describeTargets(project)
	if asJSON {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "failed to write targets: %v\n", err)
			return 2
		}
		return 0
	}
	targetNames := make([]string, 0, len(result.Targets))
	for name := range result.Targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)
	for _, name := range targetNames {
		target := result.Targets[name]
		fmt.Fprintf(stdout, "%s\tservices=%s\tphases=%s\n", name, formatTargetValues(target.Services), formatTargetValues(target.Phases))
	}
	return 0
}

func formatTargetValues(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return fmt.Sprint(values)
}
