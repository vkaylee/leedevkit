package lifecycle

import (
	"sync"
	"testing"
)

func TestBuildLockSerializesSameFingerprint(t *testing.T) {
	lock := NewBuildLock()
	started := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	first := false
	second := false
	go func() { defer wg.Done(); lock.With("same", func() { first = true; close(started); <-release }) }()
	<-started
	go func() { defer wg.Done(); lock.With("same", func() { second = true }) }()
	close(release)
	wg.Wait()
	if !first || !second {
		t.Fatalf("first=%v second=%v", first, second)
	}
}
