package testplan

import (
	"reflect"
	"strings"
	"testing"
)

func TestScheduleRejectsInvalidInput(t *testing.T) {
	for _, flags := range []Flags{
		{},
		{Target: "go", BuildPolicy: "invalid"},
		{Target: "infra", E2EOnly: true},
		{Target: "infra", LintOnly: true, UnitOnly: true},
	} {
		result := Run(flags, nil, nil)
		if result.Status != "blocked" || result.FullRegression || result.ExitCode != 2 || len(result.Blocked) != 1 || result.Blocked[0].Diagnostic == "" {
			t.Fatalf("flags=%+v result=%+v", flags, result)
		}
	}
}

func TestScheduleHonorsFocusAndSkipFlags(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags Flags
		want  []string
	}{
		{"lint", Flags{Target: "go", LintOnly: true}, []string{"skipped", "skipped", "selected"}},
		{"integration", Flags{Target: "go", E2EOnly: true}, []string{"skipped", "selected", "skipped"}},
		{"skip", Flags{Target: "go", SkipLint: true, SkipE2E: true}, []string{"selected", "skipped", "skipped"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			phases, err := Schedule(tc.flags)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, phase := range phases {
				got = append(got, phase.Status)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("statuses = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunMissingExecutorReportsBlocked(t *testing.T) {
	result := Run(Flags{Target: "infra"}, nil, nil)
	if result.Status != "blocked" || result.ExitCode == 0 || result.FullRegression || len(result.Blocked) != 1 || !strings.Contains(result.Blocked[0].Diagnostic, "executor") {
		t.Fatalf("result = %+v", result)
	}
}
