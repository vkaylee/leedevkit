package lifecycle

import "testing"

func TestBuildMetadataRequiresRebuildWhenFingerprintChanges(t *testing.T) {
	if (BuildMetadata{Expected: "new", Actual: "old"}).Fresh() {
		t.Fatal("expected stale metadata")
	}
	if (BuildMetadata{Expected: "same", Actual: "same"}).Fresh() == false {
		t.Fatal("expected fresh metadata")
	}
}

func TestBuildMetadataMissingIsStale(t *testing.T) {
	if (BuildMetadata{Expected: "x"}).Fresh() {
		t.Fatal("missing actual metadata must be stale")
	}
}
