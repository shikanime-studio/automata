package github

import (
	"context"

	update "github.com/shikanime-studio/automata/internal/updater"
)

// Updater queries GitHub to find suitable latest action tags.
type Updater struct {
	c    *Client
	opts []FindLatestOption
}

// NewUpdater constructs an Updater using the provided Client and options.
func NewUpdater(client *Client, opts ...FindLatestOption) Updater {
	return Updater{
		c:    client,
		opts: opts,
	}
}

// Update returns the latest tag for the given GitHub Action reference.
func (u Updater) Update(
	ctx context.Context,
	action *ActionRef,
	opts ...update.Option,
) (string, error) {
	return u.c.FindLatestActionTag(
		ctx,
		action,
		append(u.opts, WithUpdateOptions(opts...))...,
	)
}
