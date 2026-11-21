package lifecycle

import "testing"

func TestShouldBuildUsesFreshnessForDefaultPolicy(t *testing.T) {
	if ShouldBuild(BuildPolicy{}, BuildMetadata{Expected: "same", Actual: "same"}) {
		t.Fatal("fresh image should not build")
	}
	if !ShouldBuild(BuildPolicy{}, BuildMetadata{Expected: "new", Actual: "old"}) {
		t.Fatal("stale image should build")
	}
}

func TestSkipBuildWinsOverFreshness(t *testing.T) {
	if ShouldBuild(BuildPolicy{Build: true, SkipBuild: true}, BuildMetadata{}) {
		t.Fatal("skip-build must win")
	}
}
