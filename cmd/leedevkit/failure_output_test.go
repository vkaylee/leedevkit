package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/report"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

func TestCollectFailureDetailsUnknownLogUsesShortFallback(t *testing.T) {
	root := t.TempDir()
	log := strings.Repeat("passing output\n", 25000) + "runner crashed\n"
	if err := os.WriteFile(filepath.Join(root, "failure.log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	result := testplan.RunResult{Failed: []report.Phase{{Phase: "Unit Tests", Log: "failure.log", Diagnostic: "runner exited 1"}}}
	var output bytes.Buffer
	collectFailureDetails(root, &result, &output)
	if strings.Contains(output.String(), "passing output") || !strings.Contains(output.String(), "failure.log") || !strings.Contains(output.String(), "runner exited 1") {
		t.Fatalf("fallback output length = %d; missing diagnostic/path or includes passing output", output.Len())
	}
}

func TestCollectFailureDetailsLargeLogUsesFallback(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.log")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(16<<20 + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	result := testplan.RunResult{Failed: []report.Phase{{Phase: "Unit Tests", Log: "large.log", Diagnostic: "failed"}}}
	var output bytes.Buffer
	collectFailureDetails(root, &result, &output)
	if !strings.Contains(output.String(), "large.log") || !strings.Contains(output.String(), "exceeds parsing limit") || len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Message, "exceeds parsing limit") {
		t.Fatalf("output=%q failures=%#v", output.String(), result.Failures)
	}
}

func TestCollectFailureDetailsTextFallbackExcludesPassingOutput(t *testing.T) {
	root := t.TempDir()
	log := "--- FAIL: TestBad (0.00s)\n    bad_test.go:3: broken\npassing test output without verbose markers\n--- PASS: TestGood (0.00s)\n"
	if err := os.WriteFile(filepath.Join(root, "failure.log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	result := testplan.RunResult{Failed: []report.Phase{{Phase: "Unit Tests", Log: "failure.log", Diagnostic: "failed"}}}
	var output bytes.Buffer
	collectFailureDetails(root, &result, &output)
	if strings.Contains(output.String(), "passing test output") || !strings.Contains(output.String(), "Cannot filter") || !strings.Contains(output.String(), "failure.log") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestCollectFailureDetailsBlockedWithoutLogShowsLifecycleError(t *testing.T) {
	result := testplan.RunResult{Blocked: []report.Phase{{Phase: "Unit Tests", Diagnostic: "startup failed", LifecycleError: "port already allocated"}}}
	var output bytes.Buffer
	collectFailureDetails(t.TempDir(), &result, &output)
	if !strings.Contains(output.String(), "port already allocated") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestCollectFailureDetailsFiltersPassingTests(t *testing.T) {
	root := t.TempDir()
	log := "\x1b[32mPASS good.test.ts\x1b[0m\nFAIL src/login.test.ts > validates email\nAssertionError: expected false to be true\n    at src/login.test.ts:42:15\nPASS other.test.ts\nFAIL src/login.test.ts > password\nError: missing password\n    at src/login.test.ts:55:2\n"
	if err := os.WriteFile(filepath.Join(root, "failure.log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	result := testplan.RunResult{Failed: []report.Phase{{Phase: "Unit Tests", Log: "failure.log", Diagnostic: "failed"}}}
	var output bytes.Buffer
	collectFailureDetails(root, &result, &output)
	if len(result.Failures) != 2 || result.Failures[0].Line != 42 || result.Failures[1].TestName != "password" {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if strings.Contains(output.String(), "PASS") || strings.Contains(output.String(), "\x1b") || !strings.Contains(output.String(), "Cannot filter") || !strings.Contains(output.String(), "failure.log") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestCollectFailureDetailsSyncsPhaseFailures(t *testing.T) {
	root := t.TempDir()
	log := `{"Action":"run","Package":"example/p","Test":"TestSync"}` + "\n" +
		`{"Action":"output","Package":"example/p","Test":"TestSync","Output":"sync_test.go:19: expected ok\n"}` + "\n" +
		`{"Action":"fail","Package":"example/p","Test":"TestSync"}` + "\n" +
		`{"Action":"fail","Package":"example/p"}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "failure.log"), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	phase := report.Phase{
		Phase:      "Unit Tests",
		Status:     "failed",
		Log:        "failure.log",
		Diagnostic: "raw diagnostic string",
		Failure:    &report.Failure{Phase: "Unit Tests", Message: "raw diagnostic string"},
	}
	result := testplan.RunResult{
		Phases: []report.Phase{phase},
		Failed: []report.Phase{phase},
	}
	collectFailureDetails(root, &result, nil)
	if len(result.Failures) != 1 || result.Failures[0].TestName != "TestSync" {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if result.Phases[0].Failure == nil || result.Phases[0].Failure.TestName != "TestSync" || result.Phases[0].Failure.Line != 19 {
		t.Fatalf("Phases[0].Failure not synced: %#v", result.Phases[0].Failure)
	}
	if result.Failed[0].Failure == nil || result.Failed[0].Failure.TestName != "TestSync" || result.Failed[0].Failure.Message == "raw diagnostic string" {
		t.Fatalf("Failed[0].Failure not synced: %#v", result.Failed[0].Failure)
	}
}
