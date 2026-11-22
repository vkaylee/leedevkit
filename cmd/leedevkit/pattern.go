package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/vkaylee/leedevkit/internal/gates"
)

func patternGoGate(definitions []gates.Gate, pattern string) ([]gates.Gate, error) {
	// ponytail: subtest paths need Go's slash-aware matcher; add it before accepting slash patterns.
	if pattern == "" || strings.Contains(pattern, "/") {
		return nil, fmt.Errorf("--pattern requires a non-empty Go test regex without subtest paths")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return nil, fmt.Errorf("invalid --pattern: %w", err)
	}
	var selected []gates.Gate
	for _, gate := range definitions {
		if gate.Phase != "unit" {
			continue
		}
		// Custom commands and multiple gates cannot guarantee a single filtered execution.
		if gate.Name != "go-test" || gate.Mode != "run" || len(gate.Command) != 3 || gate.Command[0] != "go" || gate.Command[1] != "test" || gate.Command[2] != "./..." || len(selected) != 0 || len(gate.SetupCommand) != 0 || len(gate.TeardownCommand) != 0 || len(gate.Requires) != 0 || gate.ExpectedVersion != nil || len(gate.VersionCommand) != 0 || (gate.Condition != "" && gate.Condition != "always") || !gate.Required || (gate.FailurePolicy != "" && gate.FailurePolicy != "required") {
			return nil, fmt.Errorf("--pattern does not support custom unit gates")
		}
		gate.Command = []string{"go", "test", "-json", "-run=" + pattern, "./..."}
		selected = append(selected, gate)
	}
	if len(selected) != 1 {
		return nil, fmt.Errorf("--pattern requires the default go-test unit gate")
	}
	return selected, nil
}

func patternGoEvents(r io.Reader) (bool, error) {
	decoder := json.NewDecoder(bufio.NewReader(r))
	matched := false
	for {
		var event struct {
			Action string
			Test   string
		}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				return matched, nil
			}
			return false, fmt.Errorf("invalid go test event: %w", err)
		}
		if event.Test != "" && (event.Action == "run" || event.Action == "skip") {
			matched = true
		}
	}
}
