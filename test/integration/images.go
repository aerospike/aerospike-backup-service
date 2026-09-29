//go:build integration

package integration

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"golang.org/x/sync/errgroup"
)

// testImages is every image the suites start.
var testImages = []string{
	aerospikeImage,
	secretAgentImage,
	minioImage,
	fakeGCSImage,
	azuriteImage,
}

// pullImages pulls every image in testImages that is not present yet, all at once.
//
// The tests run one after another, so left to testcontainers each image would be
// pulled the first time a test starts it, one pull at a time, each inside some
// test's timing. testcontainers only pulls a missing image, so once these are
// present it starts containers from them directly.
func pullImages(ctx context.Context) error {
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return fmt.Errorf("connect to docker: %w", err)
	}
	defer func() { _ = provider.Close() }()

	group, ctx := errgroup.WithContext(ctx)

	for _, image := range testImages {
		group.Go(func() error {
			if _, err := provider.Client().ImageInspect(ctx, image); err == nil {
				return nil
			}

			if err := provider.PullImage(ctx, image); err != nil {
				return fmt.Errorf("pull %s: %w", image, err)
			}

			return nil
		})
	}

	return group.Wait()
}
