package ownership

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
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

type RuntimeSweepOptions struct {
	CLI       []string
	Namespace string
	Now       time.Time
	MaxAge    time.Duration
	Run       func(context.Context, process.Request) process.Result
	IsLocked  func(string) bool
}

func isManagedSweepProject(name string) bool {
	return isManagedSweepProjectForNamespace(name, "")
}

func isManagedSweepProjectForNamespace(name, namespace string) bool {
	if strings.HasPrefix(name, "leedevkit-test-") || strings.HasPrefix(name, "leedevkit-infra-") {
		return true
	}
	return namespace != "" && name == namespace+"-test"
}

// Sweep removes only old, identifiable, unlocked test and infra projects.
func Sweep(projects []SweepProject, now time.Time, maxAge time.Duration, cleanup func(string) error) SweepResult {
	return sweepWithNamespace(projects, "", now, maxAge, cleanup)
}

func sweepWithNamespace(projects []SweepProject, namespace string, now time.Time, maxAge time.Duration, cleanup func(string) error) SweepResult {
	var result SweepResult
	for _, project := range projects {
		if !isManagedSweepProjectForNamespace(project.Name, namespace) || project.Locked || project.Created.IsZero() || now.Sub(project.Created) < maxAge {
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

// SweepRuntime discovers and removes stale unlocked test/infra containers and networks without removing volumes.
func SweepRuntime(ctx context.Context, options RuntimeSweepOptions) SweepResult {
	if len(options.CLI) == 0 {
		return SweepResult{}
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	run := options.Run
	if run == nil {
		run = process.Run
	}
	isLocked := options.IsLocked
	if isLocked == nil {
		isLocked = IsLocked
	}
	projects, err := discoverRuntimeProjects(ctx, options.CLI, options.Namespace, run, isLocked)
	if err != nil {
		return SweepResult{Errors: []error{err}}
	}
	return sweepWithNamespace(projects, options.Namespace, now, options.MaxAge, func(project string) error {
		return cleanupRuntimeProject(ctx, options.CLI, project, run)
	})
}

func runCapture(ctx context.Context, run func(context.Context, process.Request) process.Result, args []string, timeout time.Duration) (string, process.Result) {
	var stdout, stderr bytes.Buffer
	result := run(ctx, process.Request{Args: args, Stdout: &stdout, Stderr: &stderr, Timeout: timeout})
	if len(result.Stdout) == 0 && stdout.Len() > 0 {
		result.Stdout = stdout.Bytes()
	}
	if len(result.Stderr) == 0 && stderr.Len() > 0 {
		result.Stderr = stderr.Bytes()
	}
	return string(result.Stdout), result
}

func discoverRuntimeProjects(ctx context.Context, cli []string, namespace string, run func(context.Context, process.Request) process.Result, isLocked func(string) bool) ([]SweepProject, error) {
	args := append(append([]string(nil), cli...), "ps", "-a", "--filter", "label=com.docker.compose.project", "--format", `{{.Label "com.docker.compose.project"}}|{{.CreatedAt}}`)
	output, result := runCapture(ctx, run, args, 10*time.Second)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("list compose containers: %s", result.Diagnostic())
	}
	newest := map[string]time.Time{}
	for _, line := range strings.Split(output, "\n") {
		project, rawTime, found := strings.Cut(strings.TrimSpace(line), "|")
		if !found {
			continue
		}
		project = strings.TrimSpace(project)
		created, ok := parseContainerTime(strings.TrimSpace(rawTime))
		if !isManagedSweepProjectForNamespace(project, namespace) || !ok {
			continue
		}
		if prev, exists := newest[project]; !exists || created.After(prev) {
			newest[project] = created
		}
	}
	names := make([]string, 0, len(newest))
	for name := range newest {
		names = append(names, name)
	}
	sort.Strings(names)
	projects := make([]SweepProject, 0, len(names))
	for _, name := range names {
		projects = append(projects, SweepProject{Name: name, Created: newest[name], Locked: isLocked(name)})
	}
	return projects, nil
}

func cleanupRuntimeProject(ctx context.Context, cli []string, project string, run func(context.Context, process.Request) process.Result) error {
	if err := validateProjectName(project); err != nil {
		return err
	}
	filter := "label=com.docker.compose.project=" + project
	listArgs := append(append([]string(nil), cli...), "ps", "-a", "-q", "--filter", filter)
	listedOut, listed := runCapture(ctx, run, listArgs, 10*time.Second)
	if listed.ExitCode != 0 {
		return fmt.Errorf("list project containers: %s", listed.Diagnostic())
	}
	containerIDs := nonEmptyLines(listedOut)
	if len(containerIDs) > 0 {
		rmArgs := append(append([]string(nil), cli...), "rm", "-f")
		rmArgs = append(rmArgs, containerIDs...)
		_, removed := runCapture(ctx, run, rmArgs, 15*time.Second)
		if removed.ExitCode != 0 {
			return fmt.Errorf("remove project containers: %s", removed.Diagnostic())
		}
	}
	netListArgs := append(append([]string(nil), cli...), "network", "ls", "-q", "--filter", filter)
	netOut, networks := runCapture(ctx, run, netListArgs, 10*time.Second)
	if networks.ExitCode == 0 {
		if netIDs := nonEmptyLines(netOut); len(netIDs) > 0 {
			netRmArgs := append(append([]string(nil), cli...), "network", "rm")
			netRmArgs = append(netRmArgs, netIDs...)
			_, _ = runCapture(ctx, run, netRmArgs, 10*time.Second)
		}
	}
	removeLockFile(project)
	return nil
}

func parseContainerTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	fields := strings.Fields(raw)
	if len(fields) >= 3 {
		if parsed, err := time.Parse("2006-01-02 15:04:05 -0700", fields[0]+" "+fields[1]+" "+fields[2]); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func nonEmptyLines(raw string) []string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
