package lifecycle

import "testing"

func TestFingerprintIsDeterministic(t *testing.T) {
	first := Fingerprint([]byte("compose"), []byte("dockerfile"), []byte("context"))
	second := Fingerprint([]byte("compose"), []byte("dockerfile"), []byte("context"))
	if first == "" || first != second {
		t.Fatalf("fingerprints = %q, %q", first, second)
	}
}

func TestFingerprintChangesWithInput(t *testing.T) {
	first := Fingerprint([]byte("compose"), []byte("dockerfile"), []byte("context"))
	second := Fingerprint([]byte("compose-v2"), []byte("dockerfile"), []byte("context"))
	if first == second {
		t.Fatal("changed input must change fingerprint")
	}
}
