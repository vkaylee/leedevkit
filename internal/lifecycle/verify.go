package lifecycle

import (
	"context"
	"fmt"

	"github.com/vkaylee/leedevkit/internal/process"
)

func VerifyImageFingerprint(ctx context.Context, run func(context.Context, process.Request) process.Result, engine, image, expected string) error {
	labels, err := InspectImageLabels(ctx, run, engine, image)
	if err != nil {
		return err
	}
	if !ImageLabelMatches(labels, expected) {
		return fmt.Errorf("image fingerprint mismatch for %q", image)
	}
	return nil
}
