package lifecycle

import "testing"

func TestBuildFingerprintInputsIncludesAllSources(t *testing.T) {
	if BuildFingerprintInputs([]byte("compose"), []byte("docker"), []byte("ctx")) == BuildFingerprintInputs([]byte("compose"), []byte("docker-v2"), []byte("ctx")) {
		t.Fatal("dockerfile change must invalidate fingerprint")
	}
}
