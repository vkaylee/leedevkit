package targets

import (
	"reflect"
	"testing"
)

func TestResolveExpandsAliasesAndUncoveredServices(t *testing.T) {
	got := Resolve(Request{
		Target:      "all",
		AllServices: []string{"zkteco", "api", "standalone"},
		Aliases:     []Alias{{Name: "go", Services: []string{"zkteco"}}, {Name: "api", Services: []string{"api"}}},
	})
	want := []string{"go", "api", "standalone"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestResolvePreservesAliasOrderAndDeduplicates(t *testing.T) {
	got := Resolve(Request{
		Target:      "all",
		AllServices: []string{"service", "service"},
		Aliases:     []Alias{{Name: "first", Services: []string{"service"}}, {Name: "second", Services: []string{"service"}}},
	})
	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestResolveReturnsDirectTarget(t *testing.T) {
	got := Resolve(Request{Target: "api"})
	if !reflect.DeepEqual(got, []string{"api"}) {
		t.Fatalf("targets = %#v", got)
	}
}

func TestResolveEmptyAllFallsBackToAll(t *testing.T) {
	got := Resolve(Request{Target: "all"})
	if !reflect.DeepEqual(got, []string{"all"}) {
		t.Fatalf("targets = %#v", got)
	}
}
