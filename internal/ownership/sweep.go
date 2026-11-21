package ownership

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var errSweep = errors.New("sweep cleanup failed")

type SweepProject struct {
	Name    string
	Created time.Time
	Locked  bool
}

type SweepResult struct {
	Cleaned int
	Kept    int
	Errors  []error
}

// Sweep removes only old, identifiable, unlocked test projects.
func Sweep(projects []SweepProject, now time.Time, maxAge time.Duration, cleanup func(string) error) SweepResult {
	var result SweepResult
	for _, project := range projects {
		if !strings.HasPrefix(project.Name, "leedevkit-test-") || project.Locked || project.Created.IsZero() || now.Sub(project.Created) < maxAge {
			result.Kept++
			continue
		}
		if err := cleanup(project.Name); err != nil {
			result.Kept++
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", project.Name, err))
			continue
		}
		result.Cleaned++
	}
	return result
}
