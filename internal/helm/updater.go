package helm

import (
	"context"

	update "github.com/shikanime-studio/automata/internal/updater"
)

// Updater finds latest chart versions using configured options.
type Updater struct {
	opts []FindLatestOption
}

// NewUpdater constructs an Updater with optional find-latest options.
func NewUpdater(opts ...FindLatestOption) Updater {
	return Updater{
		opts: opts,
	}
}

// Update returns the latest version for the given chart reference.
func (u Updater) Update(
	ctx context.Context,
	chart *ChartRef,
	opts ...update.Option,
) (string, error) {
	return FindLatestVersion(
		ctx,
		chart,
		append(u.opts, WithUpdateOptions(opts...))...,
	)
}
