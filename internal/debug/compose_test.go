package debug

import (
	"reflect"
	"testing"
)

func TestBuildComposeConfigArgsRequestsJSON(t *testing.T) {
	environment := Environment{ComposeProject: "fixture", ComposeFiles: []string{"compose.yml"}, Profiles: []string{"debug"}, ProjectRoot: "/repo"}
	got, err := BuildComposeConfigArgs([]string{"docker", "compose"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "-p", "fixture", "-f", "/repo/compose.yml", "--profile", "debug", "config", "--format", "json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestParseComposeConfigAndResolveNetworkNames(t *testing.T) {
	environment := Environment{
		ComposeProject: "fixture",
		Networks:       []string{"backend", "frontend"},
		Services:       map[string]Service{"api": {ID: "api", Host: "api", Aliases: []string{"api"}}},
	}
	config, err := ParseComposeConfig([]byte(`{"services":{"api":{}},"networks":{"backend":{},"frontend":{"name":"shared_frontend"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateComposeConfig(config, environment); err != nil {
		t.Fatal(err)
	}
	names, err := ResolveNetworkNames(config, environment)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"fixture_backend", "shared_frontend"}) {
		t.Fatalf("network names = %#v", names)
	}
}

func TestParseComposeConfigRejectsMalformedOutput(t *testing.T) {
	if _, err := ParseComposeConfig([]byte("not json")); err == nil {
		t.Fatal("malformed compose output accepted")
	}
	if _, err := ParseComposeConfig([]byte(`[]`)); err == nil {
		t.Fatal("non-object compose output accepted")
	}
}

func TestBuildAndParseNetworkInspect(t *testing.T) {
	got, err := BuildNetworkInspectArgs([]string{"docker"}, "fixture_backend")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"docker", "network", "inspect", "fixture_backend"}) {
		t.Fatalf("inspect args = %#v", got)
	}
	if err := ValidateNetworkOwner([]byte(`[{"Labels":{"com.docker.compose.project":"fixture"}}]`), "fixture"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNetworkOwnerFailsClosed(t *testing.T) {
	cases := [][]byte{
		[]byte(`[]`),
		[]byte(`[{"Labels":{}}]`),
		[]byte(`[{"Labels":{"com.docker.compose.project":"other"}}]`),
		[]byte(`not json`),
	}
	for _, data := range cases {
		if err := ValidateNetworkOwner(data, "fixture"); err == nil {
			t.Fatalf("network owner accepted: %s", data)
		}
	}
}
