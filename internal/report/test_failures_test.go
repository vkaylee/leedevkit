package report

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoJSONFailureAndDropPassingOutput(t *testing.T) {
	log := "" +
		`{"Action":"run","Package":"p","Test":"TestBad"}` + "\n" +
		`{"Action":"output","Package":"p","Test":"TestBad","Output":"x_test.go:12: wanted X\n"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"TestBad"}` + "\n" +
		`{"Action":"output","Package":"p","Test":"TestGood","Output":"PASS\n"}` + "\n" +
		`{"Action":"pass","Package":"p","Test":"TestGood"}` + "\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 1 || failures[0].TestName != "TestBad" || failures[0].File != "x_test.go" || failures[0].Line != 12 {
		t.Fatalf("failures=%#v", failures)
	}
	if strings.Contains(filtered, "PASS") || !strings.Contains(filtered, "wanted X") {
		t.Fatalf("filtered output=%q", filtered)
	}
}

func TestParseGoJSONInterleavedPackages(t *testing.T) {
	log := `{"Action":"output","Package":"a","Test":"TestBad","Output":"a_test.go:3: first\n"}` + "\n" +
		`{"Action":"output","Package":"b","Test":"TestGood","Output":"passing secret text\n"}` + "\n" +
		`{"Action":"output","Package":"b","Test":"TestBad","Output":"b_test.go:8: second\n"}` + "\n" +
		`{"Action":"fail","Package":"a","Test":"TestBad"}` + "\n" +
		`{"Action":"pass","Package":"b","Test":"TestGood"}` + "\n" +
		`{"Action":"fail","Package":"b","Test":"TestBad"}` + "\n" +
		`{"Action":"fail","Package":"a"}` + "\n" +
		`{"Action":"fail","Package":"b"}` + "\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 2 || failures[0].File != "a_test.go" || failures[1].File != "b_test.go" || strings.Contains(filtered, "passing secret text") {
		t.Fatalf("failures=%#v filtered=%q", failures, filtered)
	}
}

func TestParseGoJSONRealCompilerFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/broken\n\ngo 1.24\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad.go"), []byte("package broken\nvar value = missing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-json", "./...")
	cmd.Dir = root
	log, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected compilation to fail")
	}
	failures, filtered := ParseTestFailures(log, "Unit Tests")
	if len(failures) == 0 || !strings.Contains(filtered, "undefined: missing") {
		t.Fatalf("failures=%#v filtered=%q log=%s", failures, filtered, log)
	}
}

func TestParseGoJSONPreservesPackagePanicAfterFailedTest(t *testing.T) {
	log := `{"Action":"output","Package":"p","Test":"TestBad","Output":"bad_test.go:5: failed\n"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"TestBad"}` + "\n" +
		`{"Action":"output","Package":"p","Output":"panic: cleanup exploded\ngoroutine 1 [running]:\n"}` + "\n" +
		`{"Action":"fail","Package":"p"}` + "\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 2 || !strings.Contains(filtered, "cleanup exploded") {
		t.Fatalf("failures=%#v filtered=%q", failures, filtered)
	}
}

func TestParseGoJSONWithCompilerStderr(t *testing.T) {
	log := "# example/broken\n./bad.go:4: undefined: missing\n" +
		`{"Action":"start","Package":"example/broken"}` + "\n" +
		`{"Action":"fail","Package":"example/broken"}` + "\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 2 || !strings.Contains(filtered, "undefined: missing") || !strings.Contains(failures[1].Message, "undefined: missing") {
		t.Fatalf("failures=%#v filtered=%q", failures, filtered)
	}
}

func TestParseGoJSONFailureKeepsStack(t *testing.T) {
	log := `{"Action":"output","Package":"p","Test":"TestBad","Output":"bad_test.go:12: expected true\npanic: bad\ngoroutine 7 [running]:\nexample.test.TestBad()\n\t/work/bad_test.go:13 +0x20\n"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"TestBad"}` + "\n"
	failures, _ := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 1 || !strings.Contains(failures[0].Stack, "goroutine 7") || !strings.Contains(failures[0].Stack, "bad_test.go:13") {
		t.Fatalf("failures=%#v", failures)
	}
}

func TestParseGoJSONNestedFailureDoesNotDuplicateParent(t *testing.T) {
	log := `{"Action":"output","Package":"p","Test":"TestParent/child","Output":"bad_test.go:7: broken\n"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"TestParent/child"}` + "\n" +
		`{"Action":"output","Package":"p","Test":"TestParent","Output":"--- FAIL: TestParent (0.00s)\n"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"TestParent"}` + "\n" +
		`{"Action":"fail","Package":"p"}` + "\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 1 || failures[0].TestName != "TestParent/child" || !strings.Contains(filtered, "broken") {
		t.Fatalf("failures=%#v filtered=%q", failures, filtered)
	}
}

func TestParseGoJSONNestedFailurePreservesParentDiagnostic(t *testing.T) {
	log := `{"Action":"output","Package":"p","Test":"TestParent","Output":"parent_test.go:4: parent failed\n"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"TestParent/child"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"TestParent"}` + "\n"
	failures, _ := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 2 || failures[1].Message != "parent failed" {
		t.Fatalf("failures=%#v", failures)
	}
}

func TestParseGoJSONPackageFailurePreservesBuildError(t *testing.T) {
	log := `{"Action":"output","Package":"broken","Output":"./bad.go:3: undefined: x\n"}` + "\n" +
		`{"Action":"fail","Package":"broken"}` + "\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 1 || !strings.Contains(filtered, "undefined: x") {
		t.Fatalf("failures=%#v filtered=%q", failures, filtered)
	}
}

func TestParseGoTextFailures(t *testing.T) {
	log := "--- FAIL: TestBad (0.00s)\n    x_test.go:23: expected value\nPASS\n--- PASS: TestGood (0.00s)\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 1 || failures[0].TestName != "TestBad" || failures[0].File != "x_test.go" || failures[0].Line != 23 {
		t.Fatalf("failures=%#v", failures)
	}
	if strings.Contains(filtered, "PASS\\n") || !strings.Contains(filtered, "expected value") {
		t.Fatalf("filtered output=%q", filtered)
	}
}

func TestParseVitestFailuresAndStripANSI(t *testing.T) {
	log := "\x1b[31mFAIL\x1b[0m src/math.test.ts > sums\n  Error: expected 2\nPASS src/other.test.ts\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 1 || failures[0].File != "src/math.test.ts" || failures[0].TestName != "sums" {
		t.Fatalf("failures=%#v", failures)
	}
	if strings.Contains(filtered, "\x1b") || strings.Contains(filtered, "PASS src/other") || !strings.Contains(filtered, "expected 2") {
		t.Fatalf("filtered output=%q", filtered)
	}
}

func TestParseMultipleFailuresWithoutPassingOutput(t *testing.T) {
	log := "\x1b[32mPASS src/good.test.ts\x1b[0m\n" +
		"FAIL src/login.test.ts > validates email\nAssertionError: expected false to be true\n    at src/login.test.ts:42:15\n" +
		"PASS src/other.test.ts\n" +
		"FAIL src/login.test.ts > requires password\nError: missing password\n    at src/login.test.ts:55:2\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 2 || failures[0].File != "src/login.test.ts" || failures[0].Line != 42 || failures[0].Message != "AssertionError: expected false to be true" || failures[1].TestName != "requires password" {
		t.Fatalf("failures = %#v", failures)
	}
	if strings.Contains(filtered, "PASS") || strings.Contains(filtered, "\x1b") || !strings.Contains(filtered, "missing password") {
		t.Fatalf("filtered = %q", filtered)
	}
}

func TestParseUnknownLogPreservesDiagnostic(t *testing.T) {
	const log = "\x1b[32mrunner crashed\x1b[0m\n"
	failures, filtered := ParseTestFailures([]byte(log), "Unit Tests")
	if len(failures) != 0 || filtered != "runner crashed\n" {
		t.Fatalf("failures=%#v filtered=%q", failures, filtered)
	}
}
