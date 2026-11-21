package lifecycle

import "testing"

func TestBuildPolicyUsesBuildWhenRequested(t *testing.T) {
	if got := (BuildPolicy{Build: true}).Flag(); got != "--build" {
		t.Fatalf("flag = %q", got)
	}
}

func TestBuildPolicySkipsBuildWhenRequested(t *testing.T) {
	if got := (BuildPolicy{SkipBuild: true}).Flag(); got != "--no-build" {
		t.Fatalf("flag = %q", got)
	}
}

func TestBuildPolicyDefaultsToNoBuild(t *testing.T) {
	if got := (BuildPolicy{}).Flag(); got != "--no-build" {
		t.Fatalf("flag = %q", got)
	}
}
