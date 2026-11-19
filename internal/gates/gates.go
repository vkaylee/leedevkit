package gates

import (
	"fmt"
	"sort"
)

type Gate struct {
	Name    string   `json:"name"`
	Phase   string   `json:"phase"`
	Command []string `json:"command"`
	Mode    string   `json:"mode"`
}

var defaults = []Gate{
	{Name: "gofmt", Phase: "lint", Command: []string{"gofmt", "-l", "."}, Mode: "check"},
	{Name: "go-vet", Phase: "lint", Command: []string{"go", "vet", "./..."}, Mode: "run"},
	{Name: "go-test", Phase: "unit", Command: []string{"go", "test", "./..."}, Mode: "run"},
	{Name: "go-coverage", Phase: "coverage", Command: []string{"go", "test", "-coverprofile=/workspace/.test_logs/coverage-go.out", "./..."}, Mode: "run"},
}

func Defaults() []Gate {
	result := make([]Gate, len(defaults))
	copy(result, defaults)
	for i := range result {
		result[i].Command = append([]string(nil), defaults[i].Command...)
	}
	return result
}

func Select(names []string, definitions map[string]Gate) ([]Gate, error) {
	if len(names) == 0 {
		return Defaults(), nil
	}
	result := make([]Gate, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		gate, ok := definitions[name]
		if !ok {
			return nil, fmt.Errorf("quality gate %q is not defined", name)
		}
		if gate.Name == "" || gate.Phase == "" || len(gate.Command) == 0 {
			return nil, fmt.Errorf("quality gate %q is invalid", name)
		}
		result = append(result, gate)
	}
	return result, nil
}

func Names(gates []Gate) []string {
	result := make([]string, 0, len(gates))
	for _, gate := range gates {
		result = append(result, gate.Name)
	}
	sort.Strings(result)
	return result
}
