package gates

import "testing"

func TestDefaultsMatchGoCoreGates(t *testing.T) {
	got := Defaults()
	want := []string{"gofmt", "go-vet", "go-test", "go-coverage"}
	for index, name := range want {
		if got[index].Name != name {
			t.Fatalf("gate %d = %q, want %q", index, got[index].Name, name)
		}
	}
}

func TestSelectDeduplicatesAndRejectsMissing(t *testing.T) {
	selected, err := Select([]string{"go-vet", "go-vet"}, map[string]Gate{
		"go-vet": Defaults()[1],
	})
	if err != nil || len(selected) != 1 {
		t.Fatalf("selected = %#v, err = %v", selected, err)
	}
	if _, err := Select([]string{"missing"}, nil); err == nil {
		t.Fatal("expected missing gate error")
	}
}
