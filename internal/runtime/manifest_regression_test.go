package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManifestRejectsInvalidMetadataAndReportsExtras(t *testing.T) {
	root := t.TempDir()
	if result, err := VerifyManifest(root); err != nil || !result.NoManifest || result.Clean() {
		t.Fatalf("missing manifest = %+v, %v", result, err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "devkit.manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result, err := VerifyManifest(root); err != nil || !reflect.DeepEqual(result.Invalid, []string{"manifest JSON invalid"}) {
		t.Fatalf("invalid JSON = %+v, %v", result, err)
	}
	data := `{"version":"other","algorithm":"sha1","file_count":2,"files":{}}`
	if err := os.WriteFile(manifestPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra"), []byte("extra"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := VerifyManifest(root)
	wantInvalid := []string{"algorithm must be sha256", "version does not match VERSION", "file_count does not match files"}
	if err != nil || !reflect.DeepEqual(result.Invalid, wantInvalid) || !reflect.DeepEqual(result.Extra, []string{"VERSION", "extra"}) || result.Clean() {
		t.Fatalf("verification = %+v, %v", result, err)
	}
}
