package testplan

import (
	"fmt"

	"github.com/vkaylee/leedevkit/internal/targets"
)

func ExpandTargets(request Request) ([]string, error) {
	if request.Target == "" {
		return nil, fmt.Errorf("test target is required")
	}
	return targets.Resolve(targets.Request{Target: request.Target, AllServices: request.AllServices, Aliases: aliases(request.Aliases)}), nil
}

func aliases(values []Alias) []targets.Alias {
	result := make([]targets.Alias, len(values))
	for i, value := range values {
		result[i] = targets.Alias{Name: value.Name, Services: value.Services}
	}
	return result
}
