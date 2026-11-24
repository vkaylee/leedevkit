package testplan

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/vkaylee/leedevkit/internal/report"
)

func TestRunProducesSummaryAndJSONL(t *testing.T) {
	var stream bytes.Buffer
	result := Run(Flags{Target: "go", UnitOnly: true, JSONStream: true}, ExecutorFunc(func(phase Phase) PhaseResult {
		if phase.Name == "Unit Tests" {
			return PhaseResult{Status: "passed", ExitCode: 0}
		}
		return PhaseResult{Status: phase.Status, ExitCode: 0}
	}), &stream)
	if result.Status != "partial" || result.FullRegression || result.Counts.Passed != 1 || result.Counts.Skipped != 2 {
		t.Fatalf("result = %#v", result)
	}
	lines := bytes.Split(bytes.TrimSpace(stream.Bytes()), []byte{'\n'})
	if len(lines) != 8 {
		t.Fatalf("JSONL lines = %d, want 8", len(lines))
	}
	for i, line := range lines {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event["schema_version"] != float64(1) {
			t.Fatalf("event = %#v", event)
		}
		want := []string{"run_started", "phase_started", "phase_finished", "phase_started", "phase_finished", "phase_started", "phase_finished", "run_finished"}[i]
		if event["event"] != want {
			t.Fatalf("event %d = %#v, want %s", i, event, want)
		}
	}
}

func TestRunStreamIncludesFinalizedFailures(t *testing.T) {
	var stream bytes.Buffer
	Run(Flags{Target: "go", JSONStream: true}, ExecutorFunc(func(phase Phase) PhaseResult {
		return PhaseResult{Status: "failed", ExitCode: 1}
	}), &stream, func(result *RunResult) {
		result.Failures = []report.Failure{{Phase: "Unit Tests", TestName: "TestBroken"}}
	})
	lines := bytes.Split(bytes.TrimSpace(stream.Bytes()), []byte{'\n'})
	var event struct {
		Summary report.Summary `json:"summary"`
	}
	if err := json.Unmarshal(lines[len(lines)-1], &event); err != nil {
		t.Fatal(err)
	}
	if len(event.Summary.Failures) != 1 || event.Summary.Failures[0].TestName != "TestBroken" {
		t.Fatalf("summary = %#v", event.Summary)
	}
}

func TestRunRecordsPhaseTiming(t *testing.T) {
	result := Run(Flags{Target: "go", UnitOnly: true}, ExecutorFunc(func(Phase) PhaseResult {
		return PhaseResult{Status: "passed"}
	}), nil)
	if len(result.Phases) != 3 {
		t.Fatalf("phases = %#v", result.Phases)
	}
	phase := result.Phases[0]
	if phase.StartTime == "" || phase.EndTime == "" || phase.DurationS < 0 {
		t.Fatalf("phase timing = %#v", phase)
	}
}

func TestRunFailureBecomesFailedSummary(t *testing.T) {
	result := Run(Flags{Target: "go"}, ExecutorFunc(func(phase Phase) PhaseResult {
		if phase.Name == "Unit Tests" {
			return PhaseResult{Status: "failed", ExitCode: 7, Diagnostic: "unit failed"}
		}
		return PhaseResult{Status: "skipped"}
	}), nil)
	if result.Status != "failed" || result.ExitCode != 7 || len(result.Failed) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Failures) != 1 || result.Failures[0].Phase != "Unit Tests" || result.Failures[0].Message != "unit failed" {
		t.Fatalf("structured failures = %#v", result.Failures)
	}
	if len(result.Summary("go", "run").Failures) != 1 {
		t.Fatal("summary omitted structured failures")
	}
}
