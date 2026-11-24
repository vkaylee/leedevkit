package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

func readyOutput(output []byte, services []string) bool {
	if len(output) == 0 {
		return false
	}
	var records []struct {
		Service string `json:"Service"`
		State   string `json:"State"`
		Health  string `json:"Health"`
	}
	if json.Unmarshal(output, &records) != nil {
		records = nil
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			var record struct {
				Service string `json:"Service"`
				State   string `json:"State"`
				Health  string `json:"Health"`
			}
			if json.Unmarshal([]byte(line), &record) != nil || record.Service == "" {
				return false
			}
			records = append(records, record)
		}
	}
	for _, service := range services {
		ready := false
		for _, record := range records {
			if record.Service == service || strings.HasSuffix(record.Service, "-"+service+"-1") || strings.HasSuffix(record.Service, "-"+service+"-2") {
				ready = strings.EqualFold(record.State, "running") && (record.Health == "" || strings.EqualFold(record.Health, "healthy"))
			}
		}
		if !ready {
			return false
		}
	}
	return true
}

type Readiness struct {
	Run func(context.Context, process.Request) process.Result
	Dir string
	Env []string
}

func (r Readiness) Wait(ctx context.Context, base []string, services []string, timeout time.Duration) error {
	if len(base) == 0 || len(services) == 0 {
		return fmt.Errorf("readiness command and services are required")
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	run := r.Run
	if run == nil {
		run = process.Run
	}
	var lastError string
	for {
		var stdout, stderr bytes.Buffer
		result := run(ctx, process.Request{Args: append(append([]string(nil), base...), services...), Dir: r.Dir, Env: append([]string(nil), r.Env...), Stdout: &stdout, Stderr: &stderr})
		if result.ExitCode == 0 {
			output := result.Stdout
			if len(output) == 0 {
				output = stdout.Bytes()
			}
			if readyOutput(output, services) {
				return nil
			}
			lastError = ""
		} else {
			if len(result.Stderr) == 0 {
				result.Stderr = stderr.Bytes()
			}
			lastError = result.Diagnostic()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			if lastError != "" {
				return fmt.Errorf("readiness timeout for %s: %s", strings.Join(services, ","), lastError)
			}
			return fmt.Errorf("readiness timeout for %s", strings.Join(services, ","))
		case <-ticker.C:
		}
	}
}
