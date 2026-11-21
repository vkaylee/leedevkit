package dbcommands

import (
	"reflect"
	"testing"
)

func TestDieselBuildUsesComposeRunArgv(t *testing.T) {
	got, err := BuildDiesel(DieselRequest{
		Compose:     []string{"docker", "compose"},
		Project:     "leedevkit-test",
		ComposeFile: ".compose/docker-compose.test.yml",
		Root:        "/repo",
		WorkingDir:  "/repo/services/api",
		Service:     "apiserver",
		Args:        []string{"migration", "run"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker", "compose", "-p", "leedevkit-test",
		"-f", ".compose/docker-compose.test.yml", "run", "-T", "--rm",
		"--workdir", "/workspace/services/api", "--entrypoint", "diesel",
		"apiserver", "migration", "run",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestDieselUsesRootWorkingDirectory(t *testing.T) {
	got, err := BuildDiesel(DieselRequest{
		Compose: []string{"compose"}, Project: "test", ComposeFile: "test.yml",
		Root: "/repo", WorkingDir: "/repo", Service: "apiserver", Args: []string{"migration", "run"},
	})
	if err != nil || got[9] != "/workspace" {
		t.Fatalf("root working directory result = %#v, err = %v", got, err)
	}
}

func TestDieselRejectsWorkingDirectoryOutsideRoot(t *testing.T) {
	_, err := BuildDiesel(DieselRequest{
		Compose: []string{"compose"}, Project: "test", ComposeFile: "test.yml",
		Root: "/repo", WorkingDir: "/tmp", Service: "apiserver", Args: []string{"migration", "run"},
	})
	if err == nil {
		t.Fatal("outside working directory accepted")
	}
}

func TestMigrationArgsUseDieselSubcommands(t *testing.T) {
	cases := map[string][]string{
		"run":    {"migration", "run"},
		"revert": {"migration", "revert"},
		"status": {"migration", "list"},
	}
	for action, want := range cases {
		got, err := MigrationArgs(action)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s args = %#v, want %#v", action, got, want)
		}
	}
}

func TestMigrationArgsRejectUnknownAction(t *testing.T) {
	if _, err := MigrationArgs("drop"); err == nil {
		t.Fatal("unknown migration action accepted")
	}
}
