package runtime

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalUpdateAcceptsVerifiedReleaseAndKeepsRunningRelease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVKIT_HOME", home)
	archive := globalUpdateArchive(t, "2.0.0")
	server := globalUpdateServer(t, archive, "valid")
	defer server.Close()
	t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)

	oldRelease := filepath.Join(home, "v1.0.0")
	writeGlobalUpdateRelease(t, oldRelease, "1.0.0")
	if err := os.Symlink("v1.0.0", filepath.Join(home, "current")); err != nil {
		t.Fatal(err)
	}

	if err := UpdateProject(t.TempDir(), "2.0.0", ""); err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}

	current, err := filepath.EvalSymlinks(filepath.Join(home, "current"))
	if err != nil || current != filepath.Join(home, "v2.0.0") {
		t.Fatalf("current resolves to %q, error = %v", current, err)
	}
	assertGlobalUpdateRelease(t, oldRelease, "1.0.0")
	newRelease := filepath.Join(home, "v2.0.0")
	binaryName, err := nativeBinaryName()
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := os.ReadFile(filepath.Join(newRelease, "bin", "leedevkit"))
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(filepath.Join(newRelease, "bin", "go", binaryName))
	if err != nil || !bytes.Equal(launcher, binary) {
		t.Fatalf("activated launcher differs from native binary: %v", err)
	}
	result, err := VerifyManifest(newRelease)
	if err != nil || len(result.Modified) != 1 || result.Modified[0] != "bin/leedevkit" || len(result.Missing)+len(result.Extra)+len(result.Invalid) != 0 {
		t.Fatalf("unexpected installed release drift: %+v, %v", result, err)
	}
}

func TestGlobalUpdateRejectsInvalidOrMissingChecksumWithoutReplacingCurrent(t *testing.T) {
	for _, checksumMode := range []string{"incorrect", "missing"} {
		t.Run(checksumMode, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("DEVKIT_HOME", home)
			oldRelease := filepath.Join(home, "v1.0.0")
			writeGlobalUpdateRelease(t, oldRelease, "1.0.0")
			if err := os.Symlink("v1.0.0", filepath.Join(home, "current")); err != nil {
				t.Fatal(err)
			}
			server := globalUpdateServer(t, globalUpdateArchive(t, "2.0.0"), checksumMode)
			defer server.Close()
			t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)

			err := UpdateProject(t.TempDir(), "2.0.0", "")
			if err == nil {
				t.Fatal("UpdateProject() succeeded without valid archive checksum")
			}
			current, resolveErr := filepath.EvalSymlinks(filepath.Join(home, "current"))
			if resolveErr != nil || current != oldRelease {
				t.Fatalf("current changed to %q, error = %v", current, resolveErr)
			}
			assertGlobalUpdateRelease(t, oldRelease, "1.0.0")
			if _, statErr := os.Lstat(filepath.Join(home, "v2.0.0")); !os.IsNotExist(statErr) {
				t.Fatalf("failed update left new release behind; stat error = %v", statErr)
			}
		})
	}
}

func TestGlobalUpdateRejectsConcurrentInstallerLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVKIT_HOME", home)
	source := filepath.Join(t.TempDir(), "release")
	writeGlobalUpdateRelease(t, source, "2.0.0")
	if err := os.Mkdir(filepath.Join(home, ".install.lock"), 0o700); err != nil {
		t.Fatal(err)
	}

	err := UpdateProject(t.TempDir(), "2.0.0", source)
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("UpdateProject() error = %v, want lock contention error", err)
	}
	if _, err := os.Lstat(filepath.Join(home, "current")); !os.IsNotExist(err) {
		t.Fatalf("lock contention changed current; lstat error = %v", err)
	}
}

func TestGlobalUpdateRejectsTraversalTargetWithoutWritingOutsideHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEVKIT_HOME", filepath.Join(home, "managed"))
	outside := filepath.Join(home, "outside")
	server := globalUpdateServer(t, globalUpdateArchive(t, "2.0.0"), "valid")
	defer server.Close()
	t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)

	err := UpdateProject(t.TempDir(), "../../outside", "")
	if err == nil {
		t.Fatal("UpdateProject() succeeded with traversal target")
	}
	if _, statErr := os.Lstat(outside); !os.IsNotExist(statErr) {
		t.Fatalf("traversal target wrote outside managed home; stat error = %v", statErr)
	}
}

func TestGlobalUpdateRejectsCurrentSymlinkOutsideManagedReleases(t *testing.T) {
	home := t.TempDir()
	managed := filepath.Join(home, "managed")
	t.Setenv("DEVKIT_HOME", managed)
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "outside-release")
	writeGlobalUpdateRelease(t, outside, "1.0.0")
	if err := os.Symlink(outside, filepath.Join(managed, "current")); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "release")
	writeGlobalUpdateRelease(t, source, "2.0.0")

	err := UpdateProject(t.TempDir(), "2.0.0", source)
	if err == nil || !strings.Contains(err.Error(), "points outside managed releases") {
		t.Fatalf("UpdateProject() error = %v, want outside-current rejection", err)
	}
	assertGlobalUpdateRelease(t, outside, "1.0.0")
	if _, err := os.Lstat(filepath.Join(managed, "v2.0.0")); !os.IsNotExist(err) {
		t.Fatalf("rejected update installed release; stat error = %v", err)
	}
}

func globalUpdateServer(t *testing.T, archive []byte, checksumMode string) *httptest.Server {
	t.Helper()
	checksum := sha256.Sum256(archive)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".tar.gz.sha256"):
			if checksumMode == "missing" {
				http.NotFound(w, r)
				return
			}
			value := hex.EncodeToString(checksum[:])
			if checksumMode == "incorrect" {
				value = strings.Repeat("0", len(value))
			}
			fmt.Fprintf(w, "%s  leedevkit-2.0.0.tar.gz\n", value)
		case strings.HasSuffix(r.URL.Path, ".tar.gz"):
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
}

func globalUpdateArchive(t *testing.T, version string) []byte {
	t.Helper()
	releaseRoot := filepath.Join(t.TempDir(), "release")
	writeGlobalUpdateRelease(t, releaseRoot, version)
	var archive bytes.Buffer
	compressed := gzip.NewWriter(&archive)
	writer := tar.NewWriter(compressed)
	err := filepath.WalkDir(releaseRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(releaseRoot, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if err := writer.WriteHeader(&tar.Header{Name: "leedevkit-" + version + "/" + name, Mode: int64(info.Mode().Perm()), Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		_, err = writer.Write(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func writeGlobalUpdateRelease(t *testing.T, root, version string) {
	t.Helper()
	for name, content := range globalUpdateFiles(t, version) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasPrefix(name, "bin/go/") {
			mode = 0o755
		}
		if err := os.WriteFile(path, content, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteManifest(root); err != nil {
		t.Fatal(err)
	}
}

func globalUpdateFiles(t *testing.T, version string) map[string][]byte {
	t.Helper()
	binaryName, err := nativeBinaryName()
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		"VERSION":                 []byte(version + "\n"),
		"sbom.cdx.json":           []byte("{}\n"),
		"bin/leedevkit":           []byte("launcher fixture\n"),
		"bin/go/" + binaryName:    []byte("native executable fixture\n"),
		".agent/rules/example.md": []byte("rule fixture\n"),
	}
}

func assertGlobalUpdateRelease(t *testing.T, root, version string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil || strings.TrimSpace(string(data)) != version {
		t.Fatalf("release VERSION = %q, error = %v; want %q", data, err, version)
	}
	result, err := VerifyManifest(root)
	if err != nil || !result.Clean() {
		t.Fatalf("release manifest result = %+v, error = %v", result, err)
	}
}
