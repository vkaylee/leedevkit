package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestProtocolCommandsRejectMalformedInputAndSchema(t *testing.T) {
	for _, command := range []string{"--plan-lifecycle", "--plan-run", "--plan-test", "--resolve-modes", "--resolve-test-targets", "--resolve-provider", "--plan-gates"} {
		for _, input := range []string{"{", `{"schema_version":2}`} {
			t.Run(command+input, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				if code := run([]string{command}, strings.NewReader(input), &stdout, &stderr); code != 2 || stderr.Len() == 0 || stdout.Len() != 0 {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
			})
		}
	}
}
