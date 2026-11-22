package runtime

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadRejectsMalformedChecksumsAndVersionMismatch(t *testing.T) {
	archive := makeReleaseArchive(t, "1.0.0")
	checksum := sha256.Sum256(archive)
	for _, tc := range []struct {
		name, checksum, want string
	}{
		{"short hash", "abcd  file.tar.gz", "checksum format invalid"},
		{"invalid hex", strings.Repeat("z", 64) + "  file.tar.gz", "checksum format invalid"},
		{"oversized", strings.Repeat("x", 1025), "checksum format invalid"},
		{"version mismatch", fmt.Sprintf("%x  file.tar.gz", checksum), "version mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".sha256") {
					fmt.Fprint(w, tc.checksum)
					return
				}
				_, _ = w.Write(archive)
			}))
			defer server.Close()
			t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)
			if release, err := Download("2.0.0"); err == nil || !strings.Contains(err.Error(), tc.want) {
				release.Close()
				t.Fatalf("download error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLatestVersionRejectsBadResponses(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body, want string
		status                        int
	}{
		{"http error", "application/json", `{}`, "HTTP", http.StatusServiceUnavailable},
		{"invalid json", "application/json", `{`, "decode latest", http.StatusOK},
		{"missing tag", "application/json", `{}`, "tag_name is missing", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			t.Setenv("LEEDEVKIT_RELEASE_BASE_URL", server.URL)
			if _, err := latestVersion(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("latest error = %v, want %q", err, tc.want)
			}
		})
	}
}
