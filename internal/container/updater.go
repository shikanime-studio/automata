package container

import (
	"context"

	"github.com/shikanime-studio/automata/internal/updater"
)

// Updater finds the latest tag for container images.
type Updater struct {
	opts []updater.Option
}

// NewUpdater creates a new Updater with optional selection options.
func NewUpdater(opts ...updater.Option) Updater {
	return Updater{
		opts: opts,
	}
}

// Update returns the latest tag for the provided image reference.
func (u Updater) Update(
	ctx context.Context,
	imageRef *ImageRef,
	opts ...updater.Option,
) (string, error) {
	return FindLatestTag(
		ctx,
		imageRef,
		WithUpdateOptions(append(u.opts, opts...)...),
	)
}
