package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalUpdateRejectsInvalidExistingReleaseWithoutChangingCurrent(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		change     func(*testing.T, string)
	}{
		{"missing manifest", "manifest required", func(t *testing.T, source string) {
			if err := os.Remove(filepath.Join(source, "devkit.manifest.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"tampered payload", "integrity check failed", func(t *testing.T, source string) {
			if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte("modified"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing native binary", "missing executable native binary", func(t *testing.T, source string) {
			name, err := nativeBinaryName()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(source, "bin", "go", name)); err != nil {
				t.Fatal(err)
			}
			if err := WriteManifest(source); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing launcher", "missing launcher", func(t *testing.T, source string) {
			if err := os.Remove(filepath.Join(source, "bin", "leedevkit")); err != nil {
				t.Fatal(err)
			}
			if err := WriteManifest(source); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, source := t.TempDir(), t.TempDir()
			t.Setenv("DEVKIT_HOME", home)
			old := filepath.Join(home, "v1.0.0")
			writeGlobalUpdateRelease(t, old, "1.0.0")
			if err := os.Symlink("v1.0.0", filepath.Join(home, "current")); err != nil {
				t.Fatal(err)
			}
			writeGlobalUpdateRelease(t, source, "2.0.0")
			tc.change(t, source)
			if err := updateGlobal(source); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("update error = %v, want %q", err, tc.want)
			}
			if got, err := os.Readlink(filepath.Join(home, "current")); err != nil || got != "v1.0.0" {
				t.Fatalf("current = %q, %v", got, err)
			}
			assertGlobalUpdateRelease(t, old, "1.0.0")
			for _, name := range []string{".install.lock", "v2.0.0"} {
				if _, err := os.Lstat(filepath.Join(home, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected %s: %v", name, err)
				}
			}
		})
	}
}
