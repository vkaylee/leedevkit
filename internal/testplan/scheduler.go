package testplan

import "fmt"

type Flags struct {
	Target      string
	BuildPolicy string
	Build       bool
	JSON        bool
	LintOnly    bool
	UnitOnly    bool
	E2EOnly     bool
	Coverage    bool
	SkipLint    bool
	SkipE2E     bool
	SkipBuild   bool
	Fix         bool
	Pattern     string
	BaseRef     string
	DryRun      bool
	JSONStream  bool
	FailedOnly  bool
	Compact     bool
}

type Phase struct {
	Name   string
	Status string
}

func Schedule(flags Flags) ([]Phase, error) {
	if flags.BuildPolicy != "" && flags.BuildPolicy != "once" && flags.BuildPolicy != "phase" {
		return nil, fmt.Errorf("test build policy must be once or phase")
	}
	if flags.Target == "" {
		return nil, fmt.Errorf("test target is required")
	}
	if flags.Target == "infra" {
		if flags.LintOnly && flags.UnitOnly {
			return nil, fmt.Errorf("infra cannot combine lint-only and unit-only")
		}
		if flags.E2EOnly {
			return nil, fmt.Errorf("infra does not support e2e-only")
		}
		phase := "Infrastructure"
		if flags.LintOnly {
			phase = "Linting"
		}
		if flags.UnitOnly {
			phase = "Unit Tests"
		}
		return []Phase{{Name: phase, Status: "selected"}}, nil
	}
	if flags.Coverage {
		return []Phase{{Name: "Coverage", Status: "selected"}}, nil
	}
	selected := map[string]bool{
		"Unit Tests":        !flags.LintOnly && !flags.E2EOnly,
		"Integration Tests": !flags.LintOnly && !flags.UnitOnly && !flags.SkipE2E,
		"Linting":           !flags.UnitOnly && !flags.E2EOnly && !flags.SkipLint,
	}
	result := make([]Phase, 0, len(selected))
	for _, name := range []string{"Unit Tests", "Integration Tests", "Linting"} {
		status := "skipped"
		if selected[name] {
			status = "selected"
		}
		result = append(result, Phase{Name: name, Status: status})
	}
	return result, nil
}
