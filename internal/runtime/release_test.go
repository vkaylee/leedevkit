package runtime

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestDownloadRejectsMissingChecksum(t *testing.T) {
	archive := makeReleaseArchive(t, "0.11.7")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, ".sha256") {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write(archive)
	}))
	defer server.Close()
	t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)
	if _, err := Download("0.11.7"); err == nil {
		t.Fatal("download accepted missing checksum")
	}
}

func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	archive := makeReleaseArchive(t, "0.11.7")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, ".sha256") {
			_, _ = io.WriteString(writer, strings.Repeat("0", sha256.Size*2)+"  release.tar.gz\n")
			return
		}
		_, _ = writer.Write(archive)
	}))
	defer server.Close()
	t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)
	if _, err := Download("0.11.7"); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("checksum mismatch error = %v", err)
	}
}

func TestDownloadAcceptsReleaseTarballFromReleaseBase(t *testing.T) {
	archive := makeReleaseArchive(t, "0.11.7")
	checksum := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/download/v0.11.7/leedevkit-0.11.7.tar.gz.sha256" {
			_, _ = fmt.Fprintf(writer, "%x  leedevkit-0.11.7.tar.gz\\n", checksum)
			return
		}
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

func TestUpdateProjectActivatesGlobalVersionAndPreservesProject(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".leedevkit")
	t.Setenv("DEVKIT_HOME", home)
	oldRelease := filepath.Join(home, "v0.11.6")
	if err := os.MkdirAll(filepath.Join(oldRelease, "bin", "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRelease, "VERSION"), []byte("0.11.6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRelease, "bin", "go", "leedevkit-go-linux-amd64"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("v0.11.6", filepath.Join(home, "current")); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\nprintf 'runtime-ok\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcherPath := filepath.Join(source, "bin", "leedevkit")
	if err := os.MkdirAll(filepath.Dir(launcherPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcherPath, []byte("#!/bin/sh\nexec \"$(dirname \"$0\")/go/"+filepath.Base(binaryPath)+"\" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(source); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, ".leedevkit")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "marker"), []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	configBefore, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateProject(root, "0.11.7", source); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(globalHome(), "current")
	if got, err := os.Readlink(current); err != nil || got != "v0.11.7" {
		t.Fatalf("global current = %q, err = %v", got, err)
	}
	command := exec.Command(filepath.Join(current, "bin", "leedevkit"), "version")
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "runtime-ok\n" {
		t.Fatalf("installed launcher output = %q, err = %v", output, err)
	}
	if got, err := os.ReadFile(filepath.Join(legacy, "marker")); err != nil || string(got) != "preserve" {
		t.Fatalf("legacy project data changed: %q, err = %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "leedevkit.toml")); err != nil || string(got) != string(configBefore) {
		t.Fatalf("project config changed: %q, err = %v", got, err)
	}
}

func TestUpdateProjectRejectsExistingReleaseWithoutDeletingIt(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".leedevkit")
	t.Setenv("DEVKIT_HOME", home)
	source := makeUpdateRelease(t, "0.11.7")
	versionDir := filepath.Join(home, "v0.11.7")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(versionDir, "user-data")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := updateGlobal(source); err == nil {
		t.Fatal("update replaced existing release")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Fatalf("existing release data removed: %q, err=%v", got, err)
	}
}

func TestUpdateProjectRejectsConcurrentInstallerLock(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".leedevkit")
	t.Setenv("DEVKIT_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".install.lock"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := updateGlobal(t.TempDir()); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("lock contention error = %v", err)
	}
}

func TestUpdateProjectRejectsMissingExecutableWithoutActivatingRelease(t *testing.T) {
	t.Setenv("DEVKIT_HOME", filepath.Join(t.TempDir(), ".leedevkit"))
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
	legacy := filepath.Join(root, ".leedevkit")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "marker"), []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateProject(root, "0.11.7", source); err == nil {
		t.Fatal("update accepted release with missing native binary")
	}
	if _, err := os.Lstat(filepath.Join(globalHome(), "current")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("global release activated on failure: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(legacy, "marker")); err != nil || string(got) != "preserve" {
		t.Fatalf("legacy runtime changed: %q, err = %v", got, err)
	}
	if got, err := os.ReadFile(configPath); err != nil || !strings.Contains(string(got), `version = "0.11.6"`) {
		t.Fatalf("project config changed on failed update: %q, err=%v", got, err)
	}
}

func makeUpdateRelease(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"bin/go", "bin", "templates", ".agent/rules", "container"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binary, err := nativeBinaryName()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "go", binary), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := "#!/bin/sh\n# leedevkit-go-linux-amd64\n"
	if err := os.WriteFile(filepath.Join(root, "bin", "leedevkit"), []byte(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(root); err != nil {
		t.Fatal(err)
	}
	return root
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
