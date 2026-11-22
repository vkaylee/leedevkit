package testplan

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFocusedSuccessfulRunsAreNotFullRegression(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags Flags
	}{
		{name: "coverage", flags: Flags{Target: "go", Coverage: true, JSONStream: true}},
		{name: "skip-build", flags: Flags{Target: "go", SkipBuild: true, JSONStream: true}},
		{name: "pattern", flags: Flags{Target: "go", Pattern: "TestRun", JSONStream: true}},
		{name: "coverage-pattern", flags: Flags{Target: "go", Coverage: true, Pattern: "TestRun", JSONStream: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stream bytes.Buffer
			result := Run(test.flags, ExecutorFunc(func(Phase) PhaseResult {
				return PhaseResult{Status: "passed", ExitCode: 0}
			}), &stream)
			wantStatus := "partial"

			if result.Status != wantStatus || result.FullRegression || result.ExitCode != 0 {
				t.Fatalf("result = %#v, want %s non-full regression with exit code 0", result, wantStatus)
			}
			lines := strings.Split(strings.TrimSpace(stream.String()), "\n")
			var finished struct {
				Event          string `json:"event"`
				FullRegression bool   `json:"full_regression"`
				Summary        struct {
					FullRegression bool `json:"full_regression"`
				} `json:"summary"`
			}
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &finished); err != nil {
				t.Fatal(err)
			}
			if finished.Event != "run_finished" || finished.FullRegression || finished.Summary.FullRegression {
				t.Fatalf("run_finished = %#v, want full_regression false in event and summary", finished)
			}
		})
	}
}

func TestFocusedPhasesRemainPartialWithZeroExitCode(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags Flags
	}{
		{name: "unit-only", flags: Flags{Target: "go", UnitOnly: true}},
		{name: "lint-only", flags: Flags{Target: "go", LintOnly: true}},
		{name: "e2e-only", flags: Flags{Target: "go", E2EOnly: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := Run(test.flags, ExecutorFunc(func(phase Phase) PhaseResult {
				return PhaseResult{Status: "passed", ExitCode: 0}
			}), nil)
			if result.Status != "partial" || result.FullRegression || result.ExitCode != 0 {
				t.Fatalf("result = %#v, want partial non-full regression with exit code 0", result)
			}
		})
	}
}
