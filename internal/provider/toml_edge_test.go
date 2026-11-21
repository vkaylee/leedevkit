package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTOMLArraysAndEscapedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo.toml")
	content := `[container]
provider = "custom"
[container.providers.custom]
cli = ["custom", "--socket", "unix:///tmp/a,b"]
compose = ["custom", "compose"]
capabilities = ["compose", "config", "up", "down", "ps"]
label = "line one # remains"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(Options{Repository: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CLI) != 3 || got.CLI[2] != "unix:///tmp/a,b" {
		t.Fatalf("cli = %#v", got.CLI)
	}
	if got.Settings["label"] != "line one # remains" {
		t.Fatalf("settings = %#v", got.Settings)
	}
}

func TestResolveTOMLMultilineSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo.toml")
	content := "[container]\nprovider = \"custom\"\n[container.providers.custom]\ncli = [\"custom\"]\ncompose = [\"custom\", \"compose\"]\ncapabilities = [\"compose\", \"config\", \"up\", \"down\", \"ps\"]\nnote = \"\"\"line one\\nline two\"\"\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(Options{Repository: path})
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings["note"] == "" {
		t.Fatalf("settings = %#v", got.Settings)
	}
}
