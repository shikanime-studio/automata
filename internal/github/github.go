package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v55/github"
	"golang.org/x/time/rate"

	"github.com/shikanime-studio/automata/internal/config"
	"github.com/shikanime-studio/automata/internal/updater"
)

// Client wraps the go-github client with a rate limiter and datastore.
type Client struct {
	c *github.Client
	l *rate.Limiter
}

// NewLimiter creates a new rate limiter for GitHub API calls.
// Authenticated: ~1.39 requests/second (5000/hour) with burst 10.
// Unauthenticated: 1 request/minute (60/hour) with burst 1.
func NewLimiter(ctx context.Context, authenticated bool) *rate.Limiter {
	if authenticated {
		limiter := rate.NewLimiter(rate.Limit(5000.0/3600.0), 10)
		slog.InfoContext(ctx, "Created authenticated GitHub rate limiter",
			"rate", "≈1.39 requests/second",
			"burst", 10)
		return limiter
	}
	limiter := rate.NewLimiter(rate.Limit(60.0/3600.0), 1)
	slog.InfoContext(ctx, "Created unauthenticated GitHub rate limiter",
		"rate", "1 request/minute",
		"burst", 1)
	return limiter
}

// NewClient creates a new GitHub client using configuration.
func NewClient(ctx context.Context, cfg *config.Config) *Client {
	tok := cfg.GitHubToken()
	if tok != "" {
		slog.InfoContext(ctx, "Using authenticated GitHub client")
		return &Client{
			c: github.NewClient(nil).WithAuthToken(tok),
			l: NewLimiter(ctx, true),
		}
	}
	slog.WarnContext(ctx, "Using unauthenticated GitHub client (rate limited)")
	return &Client{
		c: github.NewClient(nil),
		l: NewLimiter(ctx, false),
	}
}

type findLatestOptions struct {
	excludes      map[string]struct{}
	updateOptions []updater.Option
}

// FindLatestOption configures how to select the latest tag for an action.
type FindLatestOption func(*findLatestOptions)

// WithExcludes ignores any tags present in the provided set.
func WithExcludes(excludes map[string]struct{}) FindLatestOption {
	return func(o *findLatestOptions) { o.excludes = excludes }
}

// WithUpdateOptions forwards semver comparison options to the update strategy.
func WithUpdateOptions(opts ...updater.Option) FindLatestOption {
	return func(o *findLatestOptions) { o.updateOptions = opts }
}

func makeFindLatestOptions(opts ...FindLatestOption) findLatestOptions {
	o := findLatestOptions{
		excludes: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// FindLatestActionTag returns the latest tag for the given GitHub Action based on provided options.
func (gc *Client) FindLatestActionTag(
	ctx context.Context,
	action *ActionRef,
	opts ...FindLatestOption,
) (string, error) {
	if err := gc.l.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limiter: %w", err)
	}
	o := makeFindLatestOptions(opts...)
	tags, _, err := gc.c.Repositories.ListTags(ctx, action.Owner, action.Repo, nil)
	if err != nil {
		return "", fmt.Errorf("github list tags: %w", err)
	}
	bestTag := ""
	for _, t := range tags {
		if _, ok := o.excludes[*t.Name]; ok {
			slog.DebugContext(
				ctx,
				"tag excluded by exclude list",
				"tag",
				*t.Name,
				"action",
				action.String(),
				"baseline",
				action.Version,
			)
			continue
		}
		cmp, err := updater.Compare(*t.Name, action.Version, o.updateOptions...)
		if err != nil {
			return "", fmt.Errorf("failed to compare tags: %w", err)
		}
		switch cmp {
		case updater.Equal:
			bestTag = *t.Name
		case updater.Greater:
			bestTag = *t.Name
		case updater.Less:
			slog.DebugContext(
				ctx,
				"tag excluded by update strategy",
				"tag",
				*t.Name,
				"action",
				action.String(),
				"baseline",
				action.Version,
			)
		}
	}
	return bestTag, nil
}
