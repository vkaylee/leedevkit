package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkaylee/leedevkit/internal/process"
)

func TestExecuteWritesOutputLogAndReportsCleanupFailure(t *testing.T) {
	root := t.TempDir()
	result := Execute(context.Background(), ExecuteRequest{
		Plan:  Request{Compose: []string{"compose"}, Project: "leedevkit-test-logfixture", Tool: "go", Service: "go"},
		Owned: true, CleanupBase: []string{"compose", "-p", "leedevkit-test-logfixture"}, LogRoot: root, LogName: "run.log",
		RunProcess: func(_ context.Context, request process.Request) process.Result {
			fmt.Fprint(request.Stdout, "task output\n")
			fmt.Fprint(request.Stderr, "task error\n")
			return process.Result{}
		},
		RunCompose: func([]string) error { return errors.New("cleanup error") },
	})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "cleanup error") {
		t.Fatalf("result = %+v", result)
	}
	log := filepath.Join(root, ".test_logs", "leedevkit-test-logfixture", "run.log")
	data, err := os.ReadFile(log)
	if err != nil || string(data) != "task output\ntask error\n" {
		t.Fatalf("log = %q, %v", data, err)
	}
}
