package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitLocalSourceCreatesRuntimeAndPreservesMutableState(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.default.toml"), []byte("[devkit]\nversion = \"latest\"\n"), 0o644); err != nil {
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

	if err := Init(root, source, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(runtimeRoot, "VERSION")); err != nil || string(got) != "0.12.0\n" {
		t.Fatalf("installed version = %q, err = %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(runtimeRoot, "skills.d", "custom", "SKILL.md")); err != nil || string(got) != "keep\n" {
		t.Fatalf("skills state lost: %q, err = %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(runtimeRoot, ".venv")); err != nil {
		t.Fatalf("venv state lost: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, ".agent", "rules", "base.md")); err != nil || string(got) != "base\n" {
		t.Fatalf("rule sync failed: %q, err = %v", got, err)
	}
}

func TestInitCreatesAndPinsConfigFromTemplate(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.12.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "leedevkit.go.toml"), []byte("[devkit]\nversion = \"latest\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, source, false); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != "[devkit]\nversion = \"0.12.1\"\n" {
		t.Fatalf("config = %q", config)
	}
}

func TestInitPinsLatestConfigToSourceVersion(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[devkit]\nversion = \"latest\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.12.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, source, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "[devkit]\nversion = \"0.12.2\"\n" {
		t.Fatalf("config = %q", got)
	}
}

func TestUpdateProjectUsesLocalSourceAndPreservesState(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[devkit]\nversion = \"0.12.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.13.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, ".leedevkit")
	if err := os.MkdirAll(filepath.Join(target, "skills.d", "custom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "VERSION"), []byte("0.12.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "skills.d", "custom", "SKILL.md"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateProject(root, "0.13.0", source); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "VERSION")); err != nil || string(got) != "0.13.0\n" {
		t.Fatalf("updated version = %q, err = %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "skills.d", "custom", "SKILL.md")); err != nil || string(got) != "keep\n" {
		t.Fatalf("skills state lost: %q, err = %v", got, err)
	}
	config, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != "[devkit]\nversion = \"0.13.0\"\n" {
		t.Fatalf("updated config = %q", config)
	}
}

func TestInitRejectsSourceVersionMismatchBeforeReplacement(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[devkit]\nversion = \"0.12.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.13.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".leedevkit", "marker"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(root, source, false); err == nil {
		t.Fatal("version mismatch accepted")
	}
	if got, err := os.ReadFile(filepath.Join(root, ".leedevkit", "marker")); err != nil || string(got) != "old" {
		t.Fatalf("existing runtime changed: %q, err = %v", got, err)
	}
}
