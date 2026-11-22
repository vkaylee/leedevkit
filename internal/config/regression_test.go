package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsInvalidQualityConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, config, want string
	}{
		{"missing gate", "[quality.go]\nlint = ['missing']\n", "not defined"},
		{"bad phase", "[quality.gates.bad]\nphase = 'wrong'\ncommand = ['tool']\n[quality.go]\nlint = ['bad']\n", "phase"},
		{"escaping workdir", "[quality.gates.bad]\nphase = 'lint'\ncommand = ['tool']\nworking_dir = '../outside'\n[quality.go]\nlint = ['bad']\n", "working_dir"},
		{"contradictory policy", "[quality.gates.bad]\nphase = 'lint'\ncommand = ['tool']\nrequired = true\nfailure_policy = 'optional'\n[quality.go]\nlint = ['bad']\n", "optional"},
		{"bad policy", "[test]\nbuild_policy = 'unknown'\n", "build_policy"},
		{"bad toml", "[quality\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "leedevkit.toml")
			if err := os.WriteFile(path, []byte(tc.config), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestServiceForLanguageSelectsExactNameThenSortedFallback(t *testing.T) {
	project := Project{Services: map[string]Service{"z": {Language: "go"}, "a": {Language: "go"}, "web": {Language: "web"}}}
	if service, ok := project.ServiceForLanguage("go"); !ok || service.Name != "a" {
		t.Fatalf("fallback = %+v, %v", service, ok)
	}
	project.Services["go"] = Service{Language: "go"}
	if service, ok := project.ServiceForLanguage("go"); !ok || service.Name != "go" {
		t.Fatalf("exact = %+v, %v", service, ok)
	}
	if _, ok := project.ServiceForLanguage("rust"); ok {
		t.Fatal("missing service returned")
	}
}
