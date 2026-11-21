package lifecycle

import (
	"context"
	"fmt"

	"github.com/vkaylee/leedevkit/internal/process"
)

func VerifyImages(ctx context.Context, run func(context.Context, process.Request) process.Result, engine string, images []string, expected string, onImage func(string)) error {
	for _, image := range images {
		if onImage != nil {
			onImage(image)
		}
		if err := VerifyImageFingerprint(ctx, run, engine, image, expected); err != nil {
			return fmt.Errorf("verify %s: %w", image, err)
		}
	}
	return nil
}
