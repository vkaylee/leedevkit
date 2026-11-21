package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vkaylee/leedevkit/internal/process"
)

func InspectImageLabels(ctx context.Context, run func(context.Context, process.Request) process.Result, engine, image string) (map[string]string, error) {
	if run == nil {
		run = process.Run
	}
	result := run(ctx, process.Request{Args: []string{engine, "image", "inspect", "--format", "{{json .Config.Labels}}", image}})
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("inspect image: %w", result.Err)
	}
	var labels map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(stringOutput(result)))), &labels); err != nil {
		return nil, fmt.Errorf("decode image labels: %w", err)
	}
	return labels, nil
}

func stringOutput(result process.Result) []byte { return result.Stdout }
