package testplan

import (
	"reflect"
	"testing"
)

func TestPlanResolvesTargetsAndModesFromNormalizedInput(t *testing.T) {
	result, err := Plan(Request{
		Target:      "all",
		AllServices: []string{"backend", "frontend", "standalone"},
		Aliases:     []Alias{{Name: "api", Services: []string{"backend"}}, {Name: "web", Services: []string{"frontend"}}},
		Services:    []Service{{Name: "backend", Language: "go"}, {Name: "frontend", Language: "typescript"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Targets, []string{"api", "web", "standalone"}) {
		t.Fatalf("targets = %#v", result.Targets)
	}
	if result.Modes["backend"] != "go" || result.Modes["frontend"] != "web" || result.Modes["all"] != "all" {
		t.Fatalf("modes = %#v", result.Modes)
	}
}

func TestPlanPreservesDirectTarget(t *testing.T) {
	result, err := Plan(Request{Target: "go", AllServices: []string{"go"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Targets, []string{"go"}) {
		t.Fatalf("targets = %#v", result.Targets)
	}
}

func TestPlanRejectsMissingTarget(t *testing.T) {
	if _, err := Plan(Request{}); err == nil {
		t.Fatal("expected missing target error")
	}
}
