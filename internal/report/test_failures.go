package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	ansiCSI       = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiOSC       = regexp.MustCompile(`\x1b\][^\x07]*(?:\x07|\x1b\\)`)
	goLocation    = regexp.MustCompile(`^(.+\.go):(\d+):(?:\d+:)?\s*(.*)$`)
	jsLocation    = regexp.MustCompile(`(?:\bat\s+(?:.*?\s+\()?|❯\s+)([^\s()]+\.(?:[cm]?[jt]sx?)):(\d+)(?::\d+)?\)?`)
	jsError       = regexp.MustCompile(`^(?:\w*Error|AssertionError):\s*.+`)
	vitestFailure = regexp.MustCompile(`^(?:FAIL\s+|❯\s+)(.+?)(?:\s+>\s+(.+))?$`)
)

type goTestEvent struct {
	Action      string `json:"Action"`
	Package     string `json:"Package"`
	Test        string `json:"Test"`
	Output      string `json:"Output"`
	ImportPath  string `json:"ImportPath"`
	FailedBuild string `json:"FailedBuild"`
}

// ParseTestFailures extracts recognized Go or Vitest/Bun test failures. Unknown
// formats keep their complete ANSI-stripped log so diagnostics are not lost.
func ParseTestFailures(log []byte, phase string) ([]Failure, string) {
	failures, filtered, _ := ParseTestFailuresWithSupport(log, phase)
	return failures, filtered
}

// ParseTestFailuresWithSupport also reports whether compact filtering is reliable.
func ParseTestFailuresWithSupport(log []byte, phase string) ([]Failure, string, bool) {
	clean := StripANSI(string(log))
	if failures, filtered, ok := parseGoJSON(clean, phase); ok {
		return failures, filtered, true
	}
	if failures, filtered, ok := parseGoText(clean, phase); ok {
		return failures, filtered, false
	}
	if failures, filtered, ok := parseVitest(clean, phase); ok {
		return failures, filtered, false
	}
	var filtered strings.Builder
	for _, line := range strings.SplitAfter(clean, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "PASS ") || strings.HasPrefix(trimmed, "--- PASS:") || strings.HasPrefix(trimmed, "✓ ") || strings.HasPrefix(trimmed, "ok  ") || strings.HasPrefix(trimmed, "ok\t") || (strings.HasPrefix(trimmed, "#") && strings.HasSuffix(trimmed, " CACHED")) {
			continue
		}
		filtered.WriteString(line)
	}
	return nil, filtered.String(), false
}

// StripANSI removes terminal CSI and OSC control sequences from logs.
func StripANSI(value string) string {
	value = ansiOSC.ReplaceAllString(value, "")
	return ansiCSI.ReplaceAllString(value, "")
}

func parseGoJSON(log, phase string) ([]Failure, string, bool) {
	scanner := bufio.NewScanner(strings.NewReader(log))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	var events []goTestEvent
	var compilerOutput strings.Builder
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var event goTestEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Action == "" {
			line := strings.TrimSpace(scanner.Text())
			// ponytail: mixed stderr has no package attribution; keep it separately instead of guessing ownership.
			if strings.HasPrefix(line, "# ") || goLocation.MatchString(line) || strings.HasPrefix(line, "go: ") || compilerOutput.Len() > 0 && !strings.HasPrefix(line, "{") {
				compilerOutput.WriteString(scanner.Text() + "\n")
				continue
			}
			return nil, "", false
		}
		event.Output = StripANSI(event.Output)
		events = append(events, event)
	}
	if scanner.Err() != nil || len(events) == 0 {
		return nil, "", false
	}
	failed := make(map[string]bool)
	for _, event := range events {
		if event.Action == "fail" && event.Test != "" {
			failed[event.Package+"\x00"+event.Test] = true
		}
	}
	parents := make(map[string]bool)
	for _, event := range events {
		if event.Action != "fail" || event.Test == "" {
			continue
		}
		name := event.Test
		for strings.Contains(name, "/") {
			name = name[:strings.LastIndex(name, "/")]
			parents[event.Package+"\x00"+name] = true
		}
	}
	outputs := make(map[string]*strings.Builder)
	for _, event := range events {
		key := event.Package + "\x00" + event.Test
		if event.Action == "output" && failed[key] {
			if outputs[key] == nil {
				outputs[key] = &strings.Builder{}
			}
			outputs[key].WriteString(event.Output)
		}
	}
	var failures []Failure
	var filtered strings.Builder
	for _, event := range events {
		key := event.Package + "\x00" + event.Test
		if event.Test == "" || !failed[key] {
			continue
		}
		output := ""
		if outputs[key] != nil {
			output = outputs[key].String()
		}
		// ponytail: parent diagnostics cannot be attributed to child tests; keep them when non-summary output exists.
		if parents[key] && goSummaryOnly(output) {
			continue
		}
		if event.Action == "output" {
			filtered.WriteString(event.Output)
		}
		if event.Action != "fail" {
			continue
		}
		failure := Failure{Phase: phase, TestName: event.Test, Message: "test failed", Stack: strings.TrimSpace(output)}
		for _, line := range strings.Split(output, "\n") {
			match := goLocation.FindStringSubmatch(strings.TrimSpace(line))
			if len(match) == 4 {
				failure.File = match[1]
				_, _ = fmt.Sscanf(match[2], "%d", &failure.Line)
				if match[3] != "" {
					failure.Message = match[3]
				}
				break
			}
		}
		failures = append(failures, failure)
	}
	packagesWithFailedTests := make(map[string]bool)
	for _, event := range events {
		if event.Action == "fail" && event.Test != "" {
			packagesWithFailedTests[event.Package] = true
		}
	}
	buildOutput := make(map[string]*strings.Builder)
	for _, event := range events {
		if event.Action == "build-output" && event.ImportPath != "" {
			if buildOutput[event.ImportPath] == nil {
				buildOutput[event.ImportPath] = &strings.Builder{}
			}
			buildOutput[event.ImportPath].WriteString(event.Output)
		}
	}
	usedBuild := make(map[string]bool)
	compilerDiagnostic := strings.TrimSpace(compilerOutput.String())
	for _, failedPackage := range events {
		pkg := failedPackage.Package
		if failedPackage.Action != "fail" || failedPackage.Test != "" {
			continue
		}
		var detail strings.Builder
		for _, event := range events {
			if event.Package == pkg && event.Test == "" && event.Action == "output" {
				for _, line := range strings.SplitAfter(event.Output, "\n") {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "FAIL\t") || trimmed == "FAIL" || trimmed == "PASS" {
						continue
					}
					detail.WriteString(line)
				}
			}
		}
		message := strings.TrimSpace(detail.String())
		if output := buildOutput[failedPackage.FailedBuild]; output != nil && !usedBuild[failedPackage.FailedBuild] {
			message = strings.TrimSpace(output.String() + "\n" + message)
			usedBuild[failedPackage.FailedBuild] = true
		}
		if message == "" && packagesWithFailedTests[pkg] {
			continue
		}
		if message == "" {
			message = "package failed: " + pkg
		}
		failures = append(failures, Failure{Phase: phase, Message: message})
		filtered.WriteString(message + "\n")
	}
	if compilerDiagnostic != "" {
		failures = append(failures, Failure{Phase: phase, Message: compilerDiagnostic})
		filtered.WriteString(compilerDiagnostic + "\n")
	}
	return failures, filtered.String(), true
}

func goSummaryOnly(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "--- FAIL:") && !strings.HasPrefix(line, "=== RUN ") && !strings.HasPrefix(line, "=== PAUSE ") && !strings.HasPrefix(line, "=== CONT ") {
			return false
		}
	}
	return true
}

func parseGoText(log, phase string) ([]Failure, string, bool) {
	lines := strings.Split(log, "\n")
	var failures []Failure
	var block strings.Builder
	var filtered strings.Builder
	active := false
	for _, line := range lines {
		if active && (strings.HasPrefix(line, "--- PASS: ") || line == "PASS" || strings.HasPrefix(line, "=== RUN ") || strings.HasPrefix(line, "ok  ") || strings.HasPrefix(line, "FAIL\t")) {
			appendGoTextFailure(&failures, &filtered, block.String(), phase)
			block.Reset()
			active = false
			continue
		}
		if strings.HasPrefix(line, "--- FAIL: ") {
			if active {
				appendGoTextFailure(&failures, &filtered, block.String(), phase)
			}
			block.Reset()
			block.WriteString(line)
			block.WriteByte('\n')
			active = true
			continue
		}
		if active {
			block.WriteString(line)
			block.WriteByte('\n')
		}
	}
	if active {
		appendGoTextFailure(&failures, &filtered, block.String(), phase)
	}
	return failures, filtered.String(), len(failures) > 0
}

func appendGoTextFailure(failures *[]Failure, filtered *strings.Builder, block, phase string) {
	lines := strings.Split(strings.TrimSpace(block), "\n")
	if len(lines) == 0 {
		return
	}
	name := strings.TrimPrefix(strings.TrimSpace(lines[0]), "--- FAIL: ")
	if index := strings.Index(name, " ("); index >= 0 {
		name = name[:index]
	}
	failure := Failure{Phase: phase, TestName: name, Message: strings.TrimSpace(lines[0]), Stack: strings.TrimSpace(block)}
	for _, line := range lines[1:] {
		match := goLocation.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) == 4 {
			failure.File = match[1]
			_, _ = fmt.Sscanf(match[2], "%d", &failure.Line)
			if match[3] != "" {
				failure.Message = match[3]
			}
			break
		}
	}
	*failures = append(*failures, failure)
	filtered.WriteString(block)
}

func parseVitest(log, phase string) ([]Failure, string, bool) {
	lines := strings.Split(log, "\n")
	var failures []Failure
	var block strings.Builder
	var filtered strings.Builder
	active := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if active && (strings.HasPrefix(trimmed, "PASS ") || strings.HasPrefix(trimmed, "✓ ") || strings.HasPrefix(trimmed, "Test Files ") || strings.HasPrefix(trimmed, "Tests ")) {
			appendVitestFailure(&failures, &filtered, block.String(), phase)
			block.Reset()
			active = false
			continue
		}
		if strings.HasPrefix(trimmed, "FAIL ") && vitestFailure.MatchString(trimmed) {
			if active {
				appendVitestFailure(&failures, &filtered, block.String(), phase)
			}
			block.Reset()
			block.WriteString(line)
			block.WriteByte('\n')
			active = true
			continue
		}
		if active {
			block.WriteString(line)
			block.WriteByte('\n')
		}
	}
	if active {
		appendVitestFailure(&failures, &filtered, block.String(), phase)
	}
	return failures, filtered.String(), len(failures) > 0
}

func appendVitestFailure(failures *[]Failure, filtered *strings.Builder, block, phase string) {
	lines := strings.Split(strings.TrimSpace(block), "\n")
	if len(lines) == 0 {
		return
	}
	match := vitestFailure.FindStringSubmatch(strings.TrimSpace(lines[0]))
	if len(match) != 3 {
		return
	}
	failure := Failure{Phase: phase, File: strings.TrimSpace(match[1]), TestName: strings.TrimSpace(match[2]), Message: strings.TrimSpace(lines[0])}
	var stack []string
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if jsError.MatchString(trimmed) {
			failure.Message = trimmed
		}
		if location := jsLocation.FindStringSubmatch(trimmed); len(location) == 3 {
			if failure.Line == 0 {
				failure.File = location[1]
				_, _ = fmt.Sscanf(location[2], "%d", &failure.Line)
			}
			stack = append(stack, trimmed)
		}
	}
	failure.Stack = strings.Join(stack, "\n")
	*failures = append(*failures, failure)
	filtered.WriteString(block)
}
