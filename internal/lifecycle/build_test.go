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

func TestBuildPolicyDefaultsToBuildWhenMissing(t *testing.T) {
	if got := (BuildPolicy{}).Flag(); got != "" {
		t.Fatalf("flag = %q, want empty (build only when image missing)", got)
	}
}

func TestBuildPolicySkipBuildOverridesBuild(t *testing.T) {
	if got := (BuildPolicy{Build: true, SkipBuild: true}).Flag(); got != "--no-build" {
		t.Fatalf("flag = %q, want --no-build", got)
	}
}
