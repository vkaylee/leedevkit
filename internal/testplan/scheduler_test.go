package testplan

import "testing"

func TestScheduleSelectsPhaseOrder(t *testing.T) {
	phases, err := Schedule(Flags{Target: "go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Unit Tests", "Integration Tests", "Linting"}
	for index, phase := range phases {
		if phase.Name != want[index] || phase.Status != "selected" {
			t.Fatalf("phases = %#v", phases)
		}
	}
}

func TestScheduleOnlyFlagsSkipOtherPhases(t *testing.T) {
	phases, err := Schedule(Flags{Target: "go", UnitOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if phases[0].Name != "Unit Tests" || phases[0].Status != "selected" {
		t.Fatalf("first = %#v", phases[0])
	}
	for _, phase := range phases[1:] {
		if phase.Status != "skipped" {
			t.Fatalf("phase = %#v", phase)
		}
	}
}

func TestScheduleCoverageIsExclusive(t *testing.T) {
	phases, err := Schedule(Flags{Target: "go", Coverage: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) != 1 || phases[0].Name != "Coverage" {
		t.Fatalf("phases = %#v", phases)
	}
}

func TestScheduleRejectsConflictingInfraFlags(t *testing.T) {
	if _, err := Schedule(Flags{Target: "infra", LintOnly: true, UnitOnly: true}); err == nil {
		t.Fatal("expected conflicting flag error")
	}
}
