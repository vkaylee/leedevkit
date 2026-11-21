package main

import "testing"

func TestParseOptionsRequiresEnvironmentAndManifest(t *testing.T) {
	options, err := parseOptions([]string{"--environment", "fixture", "--manifest", "/workspace/leedevkit.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Environment != "fixture" || options.Manifest != "/workspace/leedevkit.toml" {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseOptionsRejectsMissingRequiredFlag(t *testing.T) {
	if _, err := parseOptions([]string{"--environment", "fixture"}); err == nil {
		t.Fatal("missing manifest accepted")
	}
}
