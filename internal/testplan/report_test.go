package testplan

import (
	"testing"

	"github.com/vkaylee/leedevkit/internal/report"
)

func TestRunResultBuildsMachineSummary(t *testing.T) {
	result := RunResult{
		Status:         "partial",
		FullRegression: false,
		ExitCode:       0,
		Counts:         report.Counts{Passed: 1, Skipped: 2},
		Phases: []report.Phase{
			{Phase: "Unit Tests", Target: "go", Status: "passed", Required: true},
			{Phase: "Integration Tests", Target: "go", Status: "skipped", Required: false},
		},
	}
	summary := result.Summary("go", "run-1")
	if summary.SchemaVersion != 1 || summary.RunID != "run-1" || summary.Status != "partial" {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.Target != "go" || summary.Counts != result.Counts || len(summary.Phases) != 2 {
		t.Fatalf("summary fields = %#v", summary)
	}
	if !summary.Phases[0].Required || summary.Phases[1].Required {
		t.Fatalf("phase required flags = %#v", summary.Phases)
	}
	if summary.NextAction == "" {
		t.Fatal("partial summary missing next action")
	}
}

func TestRunResultSummaryUsesFailureAction(t *testing.T) {
	result := RunResult{Status: "failed", ExitCode: 7, Failed: []report.Phase{{Phase: "Unit Tests", Status: "failed"}}}
	summary := result.Summary("go", "run-2")
	if summary.NextAction == "" || summary.ExitCode != 7 || len(summary.Failed) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}
