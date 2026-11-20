package gates

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type Gate struct {
	Name            string   `json:"name"`
	Phase           string   `json:"phase"`
	Command         []string `json:"command"`
	Mode            string   `json:"mode"`
	WorkingDir      string   `json:"working_dir"`
	Service         string   `json:"service"`
	Timeout         *float64 `json:"timeout"`
	Required        bool     `json:"required"`
	Condition       string   `json:"condition"`
	FailurePolicy   string   `json:"failure_policy"`
	Environment     []string `json:"environment"`
	Requires        []string `json:"requires"`
	ExpectedVersion *string  `json:"expected_version"`
	VersionCommand  []string `json:"version_command"`
	FixCommand      []string `json:"fix_command"`
	NewFindingsFlag []string `json:"new_findings_flag"`
	SetupCommand    []string `json:"setup_command"`
	TeardownCommand []string `json:"teardown_command"`
}

var defaults = []Gate{
	defaultGate("gofmt", "lint", []string{"gofmt", "-l", "."}, "check"),
	defaultGate("go-vet", "lint", []string{"go", "vet", "./..."}, "run"),
	defaultGate("go-test", "unit", []string{"go", "test", "./..."}, "run"),
	defaultGate("go-coverage", "coverage", []string{"go", "test", "-coverprofile=/workspace/.test_logs/coverage-go.out", "./..."}, "run"),
}

func defaultGate(name, phase string, command []string, mode string) Gate {
	required := true
	condition := "always"
	failurePolicy := "required"
	return Gate{
		Name:          name,
		Phase:         phase,
		Command:       command,
		Mode:          mode,
		Timeout:       nil,
		Required:      required,
		Condition:     condition,
		FailurePolicy: failurePolicy,
		Environment:   []string{},
		Requires:      []string{},
	}
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
		gate = normalize(gate)
		if err := Validate(gate); err != nil {
			return nil, fmt.Errorf("quality gate %q is invalid: %w", name, err)
		}
		result = append(result, gate)
	}
	return result, nil
}

func normalize(gate Gate) Gate {
	if gate.Mode == "" {
		gate.Mode = "run"
	}
	if gate.Condition == "" {
		gate.Condition = "always"
	}
	if gate.FailurePolicy == "" {
		gate.FailurePolicy = "required"
	}
	if gate.Environment == nil {
		gate.Environment = []string{}
	}
	if gate.Requires == nil {
		gate.Requires = []string{}
	}
	return gate
}

func Validate(gate Gate) error {
	if strings.TrimSpace(gate.Name) == "" {
		return fmt.Errorf("name must be non-empty")
	}
	if !contains([]string{"lint", "unit", "integration", "generated", "coverage"}, gate.Phase) {
		return fmt.Errorf("phase must be supported")
	}
	if len(gate.Command) == 0 {
		return fmt.Errorf("command must be non-empty")
	}
	for _, part := range gate.Command {
		if part == "" {
			return fmt.Errorf("command must contain non-empty strings")
		}
	}
	if gate.Mode != "run" && gate.Mode != "check" && gate.Mode != "fix" {
		return fmt.Errorf("mode must be run, check, or fix")
	}
	if gate.WorkingDir != "" && (filepath.IsAbs(gate.WorkingDir) || slicesContain(filepath.Clean(gate.WorkingDir), "..")) {
		return fmt.Errorf("working_dir must stay inside project")
	}
	if gate.FailurePolicy != "required" && gate.FailurePolicy != "optional" && gate.FailurePolicy != "block" {
		return fmt.Errorf("failure_policy must be supported")
	}
	if gate.FailurePolicy == "optional" && gate.Required {
		return fmt.Errorf("required must be false for optional failure_policy")
	}
	if strings.TrimSpace(gate.Condition) == "" {
		return fmt.Errorf("condition must be non-empty")
	}
	for _, name := range gate.Environment {
		if name == "" || strings.Contains(name, "=") {
			return fmt.Errorf("environment must contain variable names")
		}
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func slicesContain(path, wanted string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part == wanted {
			return true
		}
	}
	return false
}

func Names(gates []Gate) []string {
	result := make([]string, 0, len(gates))
	for _, gate := range gates {
		result = append(result, gate.Name)
	}
	sort.Strings(result)
	return result
}
