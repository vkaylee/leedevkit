package lifecycle

import "testing"

func TestBuildMetadataRoundTrip(t *testing.T) {
	root := t.TempDir()
	metadata := BuildMetadata{Expected: "expected", Actual: "actual"}
	if err := WriteMetadata(root, "project", metadata); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMetadata(root, "project")
	if err != nil {
		t.Fatal(err)
	}
	if got != metadata {
		t.Fatalf("metadata = %#v, want %#v", got, metadata)
	}
}
