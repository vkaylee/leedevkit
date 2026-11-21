package ownership

import (
	"errors"
	"fmt"
	"time"
)

type CleanupResult struct {
	Attempts int
	Err      error
}

// Cleanup performs bounded Compose teardown without removing volumes.
func Cleanup(base []string, execute func([]string) error) CleanupResult {
	if len(base) == 0 {
		return CleanupResult{Err: errors.New("compose command is required")}
	}
	if contains(base, "-v") || contains(base, "--volumes") {
		return CleanupResult{Err: errors.New("automatic cleanup cannot remove volumes")}
	}
	down := append(append([]string(nil), base...), "down", "--remove-orphans", "--timeout", "1")
	var last error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := execute(down); err == nil {
			return CleanupResult{Attempts: attempt}
		} else {
			last = err
		}
		if attempt == 1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	fallback := append(append([]string(nil), base...), "rm", "--force", "--stop", "--no-deps")
	if err := execute(fallback); err != nil {
		return CleanupResult{Attempts: 3, Err: fmt.Errorf("teardown failed: %w; fallback failed: %v", last, err)}
	}
	return CleanupResult{Attempts: 3, Err: last}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
