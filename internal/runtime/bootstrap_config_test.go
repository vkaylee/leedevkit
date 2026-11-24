package runtime

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBootstrapIgnoresProjectVersionAndPreservesConfig(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("bootstrap supports Linux and macOS")
	}
	arch := map[string]string{"amd64": "amd64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" {
		t.Skip("unsupported bootstrap architecture")
	}
	root := t.TempDir()
	archive, err := os.Create(filepath.Join(root, "release.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(archive)
	tarball := tar.NewWriter(gz)
	for name, content := range map[string]string{
		"leedevkit-2.0.0/VERSION":                                          "2.0.0\n",
		"leedevkit-2.0.0/bin/leedevkit":                                    "launcher",
		"leedevkit-2.0.0/bin/go/leedevkit-go-" + runtime.GOOS + "-" + arch: "binary",
	} {
		if err := tarball.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarball.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarball.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	curl := "#!/bin/sh\ncase \"$*\" in *api.github.com*) printf '{\"tag_name\":\"v2.0.0\"}\\n';; *v2.0.0*) while [ \"$1\" != -o ]; do shift; done; cp \"$FIXTURE_ARCHIVE\" \"$2\";; *) exit 1;; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "bootstrap.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{"[devkit]\nversion = \"1.0.0\"\n[project]\nname = \"example\"\n", "[project]\nname = \"example\"\n"} {
		project := t.TempDir()
		path := filepath.Join(project, "leedevkit.toml")
		if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", script)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE_ARCHIVE="+archive.Name())
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bootstrap: %v\n%s", err, output)
		}
		if !strings.Contains(string(output), "Version: v2.0.0") {
			t.Fatalf("wrong release: %s", output)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != config {
			t.Fatalf("config changed: %q, %v", got, err)
		}
	}

	project := t.TempDir()
	if err := os.Mkdir(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "2.0.0")
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE_ARCHIVE="+archive.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap new project: %v\n%s", err, output)
	}
	config, err := os.ReadFile(filepath.Join(project, "leedevkit.toml"))
	if err != nil || strings.Contains(string(config), "[devkit]") {
		t.Fatalf("new config = %q, %v", config, err)
	}
}
