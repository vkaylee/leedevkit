package lifecycle

import "testing"

func TestImageLabelMatchesFingerprint(t *testing.T) {
	if !ImageLabelMatches(map[string]string{"com.leedevkit.build-fingerprint": "abc"}, "abc") {
		t.Fatal("expected matching label")
	}
	if ImageLabelMatches(map[string]string{"com.leedevkit.build-fingerprint": "old"}, "abc") {
		t.Fatal("expected stale label")
	}
}

func TestImageLabelMissingIsStale(t *testing.T) {
	if ImageLabelMatches(nil, "abc") {
		t.Fatal("missing label must be stale")
	}
}
