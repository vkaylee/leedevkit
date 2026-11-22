package runtime

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestGlobalUpdateContendsWithInstallerProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("installer shell script requires Unix")
	}
	if os.Getenv("LEEDEVKIT_UPDATER_LOCK_HELPER") == "1" {
		err := UpdateProject(os.Getenv("LOCK_TEST_PROJECT"), "2.0.0", os.Getenv("LOCK_TEST_SOURCE"))
		if err == nil || !strings.Contains(err.Error(), "already in progress") {
			t.Fatalf("UpdateProject() error = %v, want lock contention error", err)
		}
		return
	}
	home := t.TempDir()
	t.Setenv("DEVKIT_HOME", home)
	oldRelease := filepath.Join(home, "v1.0.0")
	writeGlobalUpdateRelease(t, oldRelease, "1.0.0")
	if err := os.Symlink("v1.0.0", filepath.Join(home, "current")); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	fakeBin := filepath.Join(work, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(work, "curl-started")
	curl := "#!/bin/sh\nprintf started > \"$LOCK_TEST_MARKER\"\nwhile [ ! -e \"$LOCK_TEST_RELEASE\" ]; do sleep 0.01; done\nexit 1\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte(curl), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(repositoryRoot(t), "install.sh"), "2.0.0")
	cmd.Env = append(os.Environ(), "DEVKIT_HOME="+home, "HOME="+work, "PATH="+fakeBin+":"+os.Getenv("PATH"), "LOCK_TEST_MARKER="+marker, "LOCK_TEST_RELEASE="+filepath.Join(work, "release-curl"))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd != nil {
			_ = os.WriteFile(filepath.Join(work, "release-curl"), nil, 0o600)
			_ = cmd.Wait()
		}
	}()
	deadline := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for installer to reach download while holding lock")
		case <-time.After(10 * time.Millisecond):
		}
	}

	source := filepath.Join(t.TempDir(), "release")
	writeGlobalUpdateRelease(t, source, "2.0.0")
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "leedevkit.toml"), []byte("[devkit]\nversion = \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	updater := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGlobalUpdateContendsWithInstallerProcess$", "-test.count=1")
	updater.Env = append(os.Environ(), "LEEDEVKIT_UPDATER_LOCK_HELPER=1", "DEVKIT_HOME="+home, "LOCK_TEST_PROJECT="+project, "LOCK_TEST_SOURCE="+source)
	output, err := updater.CombinedOutput()
	if err != nil {
		t.Fatalf("updater process failed: %v\n%s", err, output)
	}
	current, err := filepath.EvalSymlinks(filepath.Join(home, "current"))
	if err != nil || current != oldRelease {
		t.Fatalf("lock contention changed current to %q: %v", current, err)
	}
	assertGlobalUpdateRelease(t, oldRelease, "1.0.0")
	if data, err := os.ReadFile(filepath.Join(project, "leedevkit.toml")); err != nil || string(data) != "[devkit]\nversion = \"1.0.0\"\n" {
		t.Fatalf("project configuration changed: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".install.lock")); err != nil {
		t.Fatalf("installer did not hold lock during updater attempt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(work, "release-curl"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("installer unexpectedly succeeded after failed download")
	}
	cmd = nil
	if _, err := os.Lstat(filepath.Join(home, ".install.lock")); !os.IsNotExist(err) {
		t.Fatalf("installer failed to release lock: %v", err)
	}
}

func TestInstallerRejectsLockHeldByOtherProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("installer shell script requires Unix")
	}
	if os.Getenv("LEEDEVKIT_LOCK_OWNER_HELPER") == "1" {
		lock := filepath.Join(os.Getenv("DEVKIT_HOME"), ".install.lock")
		if err := os.Mkdir(lock, 0o700); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(lock)
		if err := os.WriteFile(os.Getenv("LOCK_TEST_MARKER"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := os.Stat(os.Getenv("LOCK_TEST_RELEASE")); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	home, work := t.TempDir(), t.TempDir()
	old := filepath.Join(home, "v1.0.0")
	writeGlobalUpdateRelease(t, old, "1.0.0")
	if err := os.Symlink("v1.0.0", filepath.Join(home, "current")); err != nil {
		t.Fatal(err)
	}
	marker, release := filepath.Join(work, "ready"), filepath.Join(work, "release")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	owner := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInstallerRejectsLockHeldByOtherProcess$", "-test.count=1")
	owner.Env = append(os.Environ(), "LEEDEVKIT_LOCK_OWNER_HELPER=1", "DEVKIT_HOME="+home, "LOCK_TEST_MARKER="+marker, "LOCK_TEST_RELEASE="+release)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.WriteFile(release, nil, 0o600)
		_ = owner.Wait()
	}()
	deadline := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for lock owner")
		case <-time.After(10 * time.Millisecond):
		}
	}
	installer := exec.CommandContext(ctx, "bash", filepath.Join(repositoryRoot(t), "install.sh"), "2.0.0")
	installer.Env = append(os.Environ(), "DEVKIT_HOME="+home)
	output, err := installer.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "another install is running") {
		t.Fatalf("installer result: %v, %s", err, output)
	}
	current, err := filepath.EvalSymlinks(filepath.Join(home, "current"))
	if err != nil || current != old {
		t.Fatalf("current changed to %q: %v", current, err)
	}
	assertGlobalUpdateRelease(t, old, "1.0.0")
	if _, err := os.Stat(filepath.Join(home, ".install.lock")); err != nil {
		t.Fatalf("installer removed lock: %v", err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
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
