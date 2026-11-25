package runtime

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitDoesNotCreateProjectRuntimeAndPreservesConfig(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	for _, dir := range []string{"templates", "bin/go", ".agent/rules"} {
		if err := os.MkdirAll(filepath.Join(source, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.12.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.default.toml"), []byte("[project]\nname = \"default\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.go.toml"), []byte("[project]\nname = \"go\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.rust.toml"), []byte("[project]\nname = \"rust\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".agent", "rules", "base.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "bin", "go", "leedevkit-go-linux-amd64"), []byte("binary\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[devkit]\nversion = \"0.12.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runtimeRoot := filepath.Join(root, ".leedevkit")
	if err := os.MkdirAll(filepath.Join(runtimeRoot, "skills.d", "custom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeRoot, "skills.d", "custom", "SKILL.md"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runtimeRoot, ".venv"), 0o755); err != nil {
		t.Fatal(err)
	}

	configBefore, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Init(root, source, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(runtimeRoot, "skills.d", "custom", "SKILL.md")); err != nil || string(got) != "keep\n" {
		t.Fatalf("legacy skills changed: %q, err = %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(runtimeRoot, ".venv")); err != nil {
		t.Fatalf("legacy virtualenv lost: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "leedevkit.toml")); err != nil || string(got) != string(configBefore) {
		t.Fatalf("project config changed: %q, err = %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, ".agent", "rules", "base.md")); err != nil || string(got) != "base\n" {
		t.Fatalf("rule sync failed: %q, err = %v", got, err)
	}
}

func TestInitCreatesLanguageSpecificConfigWithoutPinning(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.12.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.default.toml"), []byte("[project]\nname = \"default\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.go.toml"), []byte("[project]\nname = \"go\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.default.toml"), []byte("[project]\nname = \"default\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.go.toml"), []byte("[project]\nname = \"go\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, ".agent", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, source, false); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != "[project]\nname = \"go\"\n" {
		t.Fatalf("config = %q", config)
	}
}

func TestUpdateProjectInstallsGlobalVersionWithoutProjectMutation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVKIT_HOME", filepath.Join(home, ".leedevkit"))
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[devkit]\nversion = \"0.12.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.13.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binaryName, err := nativeBinaryName()
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(source, "bin", "go", binaryName)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho runtime-ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcherPath := filepath.Join(source, "bin", "leedevkit")
	if err := os.MkdirAll(filepath.Dir(launcherPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcherPath, []byte("#!/bin/sh\n# leedevkit-go-linux-amd64\necho runtime-ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(source); err != nil {
		t.Fatal(err)
	}
	projectConfig, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	legacyState := filepath.Join(root, ".leedevkit")
	if err := os.MkdirAll(filepath.Join(legacyState, "skills.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyState, "marker"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateProject(root, "0.13.0", source); err != nil {
		t.Fatal(err)
	}
	current, err := os.Readlink(filepath.Join(globalHome(), "current"))
	if err != nil || current != "v0.13.0" {
		t.Fatalf("current link = %q, err = %v", current, err)
	}
	command := exec.Command(filepath.Join(globalHome(), "current", "bin", "go", binaryName), "version")
	if output, err := command.CombinedOutput(); err != nil || string(output) != "runtime-ok\n" {
		t.Fatalf("global executable output = %q, err = %v", output, err)
	}
	if got, err := os.ReadFile(filepath.Join(legacyState, "marker")); err != nil || string(got) != "keep" {
		t.Fatalf("legacy runtime state changed: %q, err = %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "leedevkit.toml")); err != nil || string(got) != string(projectConfig) {
		t.Fatalf("project config changed: %q, err = %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "leedevkit")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project launcher created: %v", err)
	}
}

func TestInitPreservesLegacyProjectRuntime(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, ".agent", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[devkit]\nversion = \"0.12.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".leedevkit", "marker"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, source, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, ".leedevkit", "marker")); err != nil || string(got) != "old" {
		t.Fatalf("existing runtime changed: %q, err = %v", got, err)
	}
}

func TestSyncClaudeMDIdempotentAndDeduplicatesLegacyBlocks(t *testing.T) {
	root := t.TempDir()
	duplicated := "## LeeDevKit base context\n\nThis repository uses LeeDevKit. Before making changes, read and apply:\n`templates/CLAUDE.base.md`.\n\nThe rules below add specific constraints. Apply both.\n\n## Project Policy\n\n- Custom rule 1\n\n<!-- leedevkit:begin -->\n## LeeDevKit base context\n\nThis repository uses LeeDevKit. Before making changes, read and apply:\n`templates/CLAUDE.base.md`.\n<!-- leedevkit:end -->\n"
	claudePath := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(duplicated), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasClaudeMDIssue(duplicated) {
		t.Fatal("expected HasClaudeMDIssue=true for duplicated legacy CLAUDE.md")
	}
	for runIdx := 0; runIdx < 3; runIdx++ {
		if err := SyncClaudeMD(root); err != nil {
			t.Fatalf("run %d SyncClaudeMD failed: %v", runIdx, err)
		}
	}
	gotBytes, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	if HasClaudeMDIssue(got) {
		t.Fatalf("HasClaudeMDIssue still true after SyncClaudeMD:\n%s", got)
	}
	if count := countSubstr(got, "## LeeDevKit base context"); count != 1 {
		t.Fatalf("expected 1 base context heading, got %d:\n%s", count, got)
	}
	if count := countSubstr(got, ClaudeMarkerStart); count != 1 {
		t.Fatalf("expected 1 start marker, got %d:\n%s", count, got)
	}
	if count := countSubstr(got, ClaudeMarkerEnd); count != 1 {
		t.Fatalf("expected 1 end marker, got %d:\n%s", count, got)
	}
	if countSubstr(got, ClaudeMarkerLegacyBegin) != 0 {
		t.Fatalf("legacy begin marker remained:\n%s", got)
	}
	if countSubstr(got, "- Custom rule 1") != 1 || countSubstr(got, "The rules below add specific constraints.") != 1 {
		t.Fatalf("custom content lost:\n%s", got)
	}
}

func countSubstr(s, sub string) int {
	n := 0
	for idx := 0; ; {
		pos := indexOf(s[idx:], sub)
		if pos < 0 {
			return n
		}
		n++
		idx += pos + len(sub)
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
