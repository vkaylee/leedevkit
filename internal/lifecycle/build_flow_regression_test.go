package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildFlowFromFilesUsesFingerprintAndStopsOnMissingInput(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{"compose.yml": "services: {}", "Dockerfile": "FROM scratch", "context/source": "content"} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	builds, verified, published := 0, false, ""
	flow := BuildFlow{Runner: BuildRunner{
		Execute: func(context.Context, []string) error { builds++; return nil },
		Verify:  func(context.Context, string, string) error { verified = true; return nil },
		Publish: func(metadata BuildMetadata) error { published = metadata.Expected; return nil },
	}}
	plan := Plan{Provider: testProviderWithBuild(), Project: "test", Services: []string{"go"}, OwnsProject: true}
	if err := flow.RunFromFiles(context.Background(), plan, root, "compose.yml", "Dockerfile", "context", "docker", "image"); err != nil {
		t.Fatal(err)
	}
	if builds != 1 || !verified || published == "" {
		t.Fatalf("builds=%d verified=%v fingerprint=%q", builds, verified, published)
	}
	if err := flow.RunFromFiles(context.Background(), plan, root, "missing", "Dockerfile", "context", "docker", "image"); err == nil || !strings.Contains(err.Error(), "read build inputs") || builds != 1 {
		t.Fatalf("missing input error=%v builds=%d", err, builds)
	}
}
