package ownership

import (
	"testing"
	"time"
)

func TestSweepOnlyCleansOldUnlockedProjects(t *testing.T) {
	now := time.Unix(1000, 0)
	projects := []SweepProject{
		{Name: "leedevkit-test-old", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-infra-old", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-test-young", Created: now.Add(-5 * time.Minute)},
		{Name: "leedevkit-infra-young", Created: now.Add(-5 * time.Minute)},
		{Name: "other-old", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-test-unknown"},
		{Name: "leedevkit-test-locked", Created: now.Add(-2 * time.Hour), Locked: true},
		{Name: "leedevkit-infra-locked", Created: now.Add(-2 * time.Hour), Locked: true},
	}
	var cleaned []string
	result := Sweep(projects, now, time.Hour, func(name string) error {
		cleaned = append(cleaned, name)
		return nil
	})
	if len(cleaned) != 2 || cleaned[0] != "leedevkit-test-old" || cleaned[1] != "leedevkit-infra-old" {
		t.Fatalf("cleaned = %#v", cleaned)
	}
	if result.Kept != 6 || result.Cleaned != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSweepContinuesAfterCleanupError(t *testing.T) {
	now := time.Unix(1000, 0)
	result := Sweep([]SweepProject{
		{Name: "leedevkit-test-a", Created: now.Add(-2 * time.Hour)},
		{Name: "leedevkit-test-b", Created: now.Add(-2 * time.Hour)},
	}, now, time.Hour, func(name string) error {
		if name == "leedevkit-test-a" {
			return errSweep
		}
		return nil
	})
	if result.Cleaned != 1 || len(result.Errors) != 1 {
		t.Fatalf("result = %#v", result)
	}
}
