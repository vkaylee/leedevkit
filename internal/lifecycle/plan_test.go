package lifecycle

import (
	"reflect"
	"testing"

	"github.com/vkaylee/leedevkit/internal/provider"
)

func testProvider(wait bool) provider.Provider {
	capabilities := []string{"compose", "config", "down", "ps", "up"}
	if wait {
		capabilities = append(capabilities, "wait")
	}
	return provider.Provider{
		Name:         "docker",
		Compose:      []string{"docker", "compose"},
		Capabilities: capabilities,
	}
}

func TestUpNativeWaitUsesSingleComposeCommand(t *testing.T) {
	args, err := (Plan{
		Provider:     testProvider(true),
		Project:      "leedevkit-test-abc",
		Files:        []string{"compose.yml"},
		Profiles:     []string{"--profile", "go"},
		Services:     []string{"go"},
		Dependencies: []string{"postgres", "go"},
		Build:        false,
		NativeWait:   true,
		WaitTimeout:  120,
		OwnsProject:  true,
	}).Up()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker", "compose", "-p", "leedevkit-test-abc", "-f", "compose.yml", "--profile", "go", "up", "-d", "--no-build", "--wait", "--wait-timeout", "120", "go", "postgres"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestDownNeverRemovesVolumes(t *testing.T) {
	args, err := (Plan{Provider: testProvider(false), Project: "test", OwnsProject: true, WaitTimeout: 120}).Down()
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range args {
		if arg == "-v" || arg == "--volumes" {
			t.Fatalf("volume removal flag in %#v", args)
		}
	}
}

func TestExternalProjectCannotBeTornDown(t *testing.T) {
	args, err := (Plan{Provider: testProvider(false), Project: "external", OwnsProject: false, WaitTimeout: 120}).Down()
	if err != nil {
		t.Fatal(err)
	}
	if args != nil {
		t.Fatalf("down args = %#v", args)
	}
}

func TestMissingCapabilityFailsBeforeCommand(t *testing.T) {
	_, err := (Plan{Provider: testProvider(false), Project: "test", NativeWait: true, WaitTimeout: 120, OwnsProject: true}).Up()
	if err == nil {
		t.Fatal("expected wait capability error")
	}
}

func TestUpNoDepsAddsComposeFlag(t *testing.T) {
	args, err := (Plan{
		Provider: testProvider(false), Project: "test", Profiles: []string{"--profile", "go"},
		Services: []string{"go"}, Build: false, OwnsProject: true, NoDeps: true,
	}).Up()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, arg := range args {
		if arg == "--no-deps" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("args = %#v", args)
	}
}

func TestUpWithoutNativeWaitDoesNotRequireTimeout(t *testing.T) {
	if _, err := (Plan{Provider: testProvider(false), Project: "test", OwnsProject: true}).Up(); err != nil {
		t.Fatal(err)
	}
}
