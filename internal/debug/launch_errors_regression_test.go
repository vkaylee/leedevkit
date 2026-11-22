package debug

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchValidationFailsBeforeRunningUntrustedConfiguration(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "leedevkit.toml")
	if err := os.WriteFile(manifest, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	environment := Environment{Name: "fixture", ComposeProject: "fixture", ProjectRoot: root, SourcePath: manifest, Networks: []string{"backend"}}
	for _, tc := range []struct {
		name, config, inspect, want string
	}{
		{"malformed config", "{", "", "not valid JSON"},
		{"no services", `{"services":{},"networks":{"backend":{}}}`, "", "no services"},
		{"unowned network", `{"services":{"api":{}},"networks":{"backend":{}}}`, `[{"Labels":{"com.docker.compose.project":"other"}}]`, "not owned"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runs := 0
			err := LaunchWithValidation(context.Background(), environment, []string{"compose"}, []string{"docker"}, func(context.Context, []string) error {
				runs++
				return nil
			}, func(_ context.Context, args []string) ([]byte, error) {
				if contains(args, "config") {
					return []byte(tc.config), nil
				}
				return []byte(tc.inspect), nil
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) || runs != 0 {
				t.Fatalf("error=%v runs=%d want=%q", err, runs, tc.want)
			}
		})
	}
}

func TestLaunchCleansUpAfterRunnerError(t *testing.T) {
	root := t.TempDir()
	environment := Environment{Name: "fixture", ComposeProject: "fixture", ProjectRoot: root, SourcePath: filepath.Join(root, "leedevkit.toml")}
	failure := errors.New("debug run failed")
	var calls [][]string
	err := Launch(context.Background(), environment, []string{"compose"}, func(_ context.Context, args []string) error {
		calls = append(calls, args)
		if len(calls) == 1 {
			return failure
		}
		return nil
	})
	if !errors.Is(err, failure) || len(calls) != 3 || !contains(calls[1], "rm") || !contains(calls[2], "ps") {
		t.Fatalf("error=%v calls=%q", err, calls)
	}
}
