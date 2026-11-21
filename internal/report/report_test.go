package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSummaryEncodesStableMachineContract(t *testing.T) {
	report := Summary{
		RunID:          "run-1",
		Status:         "partial",
		FullRegression: false,
		ExitCode:       0,
		Target:         "go",
		Counts:         Counts{Passed: 1, Failed: 0, Blocked: 0, Skipped: 1},
		Phases:         []Phase{{Phase: "Linting", Target: "go", Status: "passed", Required: true}},
		NextAction:     "Run without focused or skip flags for full regression verification.",
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "run_id", "status", "full_regression", "exit_code", "target", "counts", "phases", "failed", "blocked", "skipped", "next_action"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing report field %q: %s", key, encoded)
		}
	}
	if decoded["schema_version"] != float64(1) || decoded["status"] != "partial" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestStreamWritesOrderedJSONLEvents(t *testing.T) {
	var output bytes.Buffer
	stream, err := NewStream(&output, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Emit("run_started", map[string]any{"target": "all"}); err != nil {
		t.Fatal(err)
	}
	if err := stream.Emit("run_finished", map[string]any{"status": "passed"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %#v", lines)
	}
	for index, line := range lines {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.SchemaVersion != 1 || event.RunID != "run-1" || event.Sequence != index+1 {
			t.Fatalf("event = %#v", event)
		}
	}
}

func TestStreamRejectsNilWriter(t *testing.T) {
	if _, err := NewStream(nil, "run"); err == nil {
		t.Fatal("expected nil writer error")
	}
}
