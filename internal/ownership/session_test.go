package ownership

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewProjectNameUsesSafePrefixAndUniqueSuffix(t *testing.T) {
	first, err := NewProjectName("leedevkit-test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewProjectName("leedevkit-test")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "leedevkit-test-") || !strings.HasPrefix(second, "leedevkit-test-") {
		t.Fatalf("names = %q, %q", first, second)
	}
}

func TestOpenOwnedProjectRejectsConcurrentOwner(t *testing.T) {
	project := "leedevkit-test-ownership-collision"
	first, err := Open(project, true)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close(nil)

	if _, err := Open(project, true); err == nil {
		t.Fatal("expected lock collision")
	}
}

func TestExternalProjectCloseNeverRunsTeardown(t *testing.T) {
	session, err := Open("leedevkit-dev", false)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	if err := session.Close(func() error {
		calls.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatalf("teardown calls = %d, want 0", calls.Load())
	}
}

func TestOwnedCloseRunsTeardownOnceAndReleasesLock(t *testing.T) {
	project := "leedevkit-test-owned-close"
	session, err := Open(project, true)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	teardownErr := errors.New("compose failed")
	if err := session.Close(func() error {
		calls.Add(1)
		return teardownErr
	}); !errors.Is(err, teardownErr) {
		t.Fatalf("close error = %v, want %v", err, teardownErr)
	}
	if err := session.Close(func() error {
		calls.Add(1)
		return nil
	}); !errors.Is(err, teardownErr) {
		t.Fatalf("second close error = %v, want %v", err, teardownErr)
	}
	if calls.Load() != 1 {
		t.Fatalf("teardown calls = %d, want 1", calls.Load())
	}

	reopened, err := Open(project, true)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	reopened.Close(nil)
}

func TestOpenRejectsInvalidProjectName(t *testing.T) {
	for _, project := range []string{"", ".", "../outside", "bad/name", "bad name"} {
		if _, err := Open(project, true); err == nil {
			t.Fatalf("project %q accepted", project)
		}
	}
}

func TestIsLockedReflectsActiveSessionAndReleasesOnClose(t *testing.T) {
	project := "leedevkit-test-islocked-check"
	if IsLocked(project) {
		t.Fatal("unlocked project reported locked")
	}
	if IsLocked("../bad") {
		t.Fatal("invalid project reported locked")
	}
	session, err := Open(project, true)
	if err != nil {
		t.Fatal(err)
	}
	if !IsLocked(project) {
		session.Close(nil)
		t.Fatal("active owned project reported unlocked")
	}
	if err := session.Close(nil); err != nil {
		t.Fatal(err)
	}
	if IsLocked(project) {
		t.Fatal("closed project still reported locked")
	}
}
