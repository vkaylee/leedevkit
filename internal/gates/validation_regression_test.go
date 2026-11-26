package gates

import (
	"strings"
	"testing"
)

func TestSelectRejectsUnsafeGateDefinitions(t *testing.T) {
	valid := Gate{Name: "check", Phase: "lint", Command: []string{"tool"}}
	version := "1.0"
	for _, tc := range []struct {
		name string
		edit func(*Gate)
		want string
	}{
		{"empty name", func(g *Gate) { g.Name = "" }, "name"},
		{"unknown phase", func(g *Gate) { g.Phase = "other" }, "phase"},
		{"empty command", func(g *Gate) { g.Command = nil }, "command"},
		{"blank argv", func(g *Gate) { g.Command = []string{"tool", ""} }, "command"},
		{"unknown mode", func(g *Gate) { g.Mode = "other" }, "mode"},
		{"escaping workdir", func(g *Gate) { g.WorkingDir = "../outside" }, "working_dir"},
		{"optional required", func(g *Gate) { g.FailurePolicy = "optional"; g.Required = true }, "required"},
		{"incomplete version", func(g *Gate) { g.ExpectedVersion = &version }, "version_command"},
		{"unsupported condition", func(g *Gate) { g.Condition = "sometimes" }, "condition"},
		{"invalid environment", func(g *Gate) { g.Environment = []string{"SECRET=value"} }, "environment"},
		{"invalid env key", func(g *Gate) { g.Env = map[string]string{"BAD=KEY": "1"} }, "env"},
		{"nul in env value", func(g *Gate) { g.Env = map[string]string{"GOOD_KEY": "a\x00b"} }, "NUL"},
		{"invalid required_env", func(g *Gate) { g.RequiredEnv = []string{"BAD=NAME"} }, "required_env"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := valid
			tc.edit(&gate)
			_, err := Select([]string{"check"}, map[string]Gate{"check": gate})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestDefaultGateCommandsAreIndependentCopies(t *testing.T) {
	first := Defaults()
	original := first[0].Command[0]
	first[0].Command[0] = "changed"
	if got := Defaults()[0].Command[0]; got != original {
		t.Fatalf("default command mutated: %q", got)
	}
}
