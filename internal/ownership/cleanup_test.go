package ownership

import (
	"errors"
	"reflect"
	"testing"
)

func TestCleanupRetriesAndNeverRemovesVolumes(t *testing.T) {
	var calls [][]string
	result := Cleanup([]string{"docker", "compose", "-p", "owned"}, func(args []string) error {
		calls = append(calls, append([]string(nil), args...))
		if len(calls) == 1 {
			return errors.New("compose down failed")
		}
		return nil
	})
	if result.Err != nil || result.Attempts != 2 {
		t.Fatalf("result = %#v", result)
	}
	for _, args := range calls {
		if hasArg(args, "-v") || hasArg(args, "--volumes") {
			t.Fatalf("volume removal in cleanup args: %#v", args)
		}
	}
	want := []string{"docker", "compose", "-p", "owned", "down", "--remove-orphans", "--timeout", "1"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("args = %#v, want %#v", calls[0], want)
	}
}

func TestCleanupFallbackUsesRmWithoutVolumes(t *testing.T) {
	var calls [][]string
	result := Cleanup([]string{"docker", "compose", "-p", "owned"}, func(args []string) error {
		calls = append(calls, append([]string(nil), args...))
		return errors.New("compose failed")
	})
	if result.Err == nil || len(calls) != 3 {
		t.Fatalf("result = %#v, calls = %#v", result, calls)
	}
	want := []string{"docker", "compose", "-p", "owned", "rm", "--force", "--stop", "--no-deps"}
	if !reflect.DeepEqual(calls[2], want) {
		t.Fatalf("fallback = %#v, want %#v", calls[2], want)
	}
}

func TestCleanupRejectsVolumeRemovalInput(t *testing.T) {
	result := Cleanup([]string{"docker", "compose", "-p", "owned", "down", "-v"}, func([]string) error { return nil })
	if result.Err == nil {
		t.Fatal("expected volume removal rejection")
	}
}

func hasArg(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
