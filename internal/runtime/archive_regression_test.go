package runtime

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractArchiveRejectsUnsafeMembersBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header tar.Header
		want   string
	}{
		{"parent", tar.Header{Name: "../escape", Mode: 0o644, Typeflag: tar.TypeReg}, "unsafe release member"},
		{"absolute", tar.Header{Name: "/escape", Mode: 0o644, Typeflag: tar.TypeReg}, "unsafe release member"},
		{"symlink", tar.Header{Name: "link", Linkname: "../outside", Mode: 0o777, Typeflag: tar.TypeSymlink}, "unsupported release member"},
		{"hardlink", tar.Header{Name: "link", Linkname: "file", Mode: 0o644, Typeflag: tar.TypeLink}, "unsupported release member"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archiveDir, stage := t.TempDir(), t.TempDir()
			archive := filepath.Join(archiveDir, "archive.tar.gz")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			compressed := gzip.NewWriter(file)
			writer := tar.NewWriter(compressed)
			if err := writer.WriteHeader(&tc.header); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := compressed.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := extractArchive(archive, stage); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("extract error = %v, want %q", err, tc.want)
			}
			entries, err := os.ReadDir(stage)
			if err != nil || len(entries) != 0 {
				t.Fatalf("stage changed: %v, %v", entries, err)
			}
		})
	}
}
