package runtime

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatestReleaseAPIUsesGitHubJSONEndpoint(t *testing.T) {
	if got, want := latestReleaseAPI("https://github.com/vkaylee/leedevkit/releases"), defaultReleaseAPI+"/latest"; got != want {
		t.Fatalf("latest API = %q, want %q", got, want)
	}
	if got, want := latestReleaseAPI("https://api.github.com/repos/example/project/releases"), "https://api.github.com/repos/example/project/releases/latest"; got != want {
		t.Fatalf("mirror API = %q, want %q", got, want)
	}
}

func TestLatestVersionReadsJSONFromReleaseAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/latest" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept = %q", request.Header.Get("Accept"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":"v0.11.7"}`))
	}))
	defer server.Close()
	t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)

	got, err := latestVersion()
	if err != nil || got != "0.11.7" {
		t.Fatalf("latestVersion() = %q, %v", got, err)
	}
}

func TestLatestVersionRejectsHTMLWithoutEchoingBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("<html>private@example.test login</html>"))
	}))
	defer server.Close()
	t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)

	_, err := latestVersion()
	if err == nil || !strings.Contains(err.Error(), "unexpected content type") {
		t.Fatalf("latestVersion error = %v", err)
	}
	if strings.Contains(err.Error(), "private@example.test") || strings.Contains(err.Error(), "<html>") {
		t.Fatalf("error leaked response body: %v", err)
	}
}

func TestDownloadAcceptsReleaseTarballFromReleaseBase(t *testing.T) {
	archive := makeReleaseArchive(t, "0.11.7")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/download/v0.11.7/leedevkit-0.11.7.tar.gz" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/gzip")
		_, _ = writer.Write(archive)
	}))
	defer server.Close()
	t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)

	release, err := Download("0.11.7")
	if err != nil {
		t.Fatal(err)
	}
	defer release.Close()
	if got, err := readVersion(filepath.Join(release.Root, "VERSION")); err != nil || got != "0.11.7" {
		t.Fatalf("download version = %q, err = %v", got, err)
	}
}

func TestUpdateProjectReplacesProjectLauncherWithSelectedNativeBinary(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "leedevkit.toml"), []byte("[devkit]\nversion = \"0.11.6\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.11.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(source, "bin", "go", "leedevkit-go-linux-amd64")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("new-native-runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "leedevkit")
	if err := os.WriteFile(launcher, []byte("old-embedded-runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	runtimeDir := filepath.Join(root, ".leedevkit")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "VERSION"), []byte("0.11.6\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateProject(root, "0.11.7", source); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-native-runtime" {
		t.Fatalf("launcher = %q; expected release Go binary", got)
	}
	if info, err := os.Stat(launcher); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("launcher mode = %v, err = %v", info, err)
	}
}

func TestUpdateProjectKeepsRuntimeAndLauncherWhenLauncherIsMissingFromRelease(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	configPath := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(configPath, []byte("[devkit]\nversion = \"0.11.6\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("0.11.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "bin", "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "leedevkit")
	if err := os.WriteFile(launcher, []byte("old-launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	runtimeDir := filepath.Join(root, ".leedevkit")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "VERSION"), []byte("0.11.6\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateProject(root, "0.11.7", source); err == nil {
		t.Fatal("update accepted release with missing native binary")
	}
	if got, err := os.ReadFile(launcher); err != nil || string(got) != "old-launcher" {
		t.Fatalf("launcher changed on failed update: %q, err = %v", got, err)
	}
	if got, err := readVersion(filepath.Join(runtimeDir, "VERSION")); err != nil || got != "0.11.6" {
		t.Fatalf("runtime changed on failed update: version=%q, err=%v", got, err)
	}
	if got, err := os.ReadFile(configPath); err != nil || !strings.Contains(string(got), `version = "0.11.6"`) {
		t.Fatalf("config changed on failed update: %q, err=%v", got, err)
	}
}

func TestWriteAtomicReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "new" {
		t.Fatalf("contents = %q, err = %v", got, err)
	}
}

func makeReleaseArchive(t *testing.T, version string) []byte {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "release.tar.gz")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	tarball := tar.NewWriter(compressed)
	if err := tarball.WriteHeader(&tar.Header{Name: "leedevkit-" + version, Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if err := tarball.WriteHeader(&tar.Header{Name: "leedevkit-" + version + "/VERSION", Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarball.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tarball.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
