package container

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"

	"github.com/shikanime-studio/automata/internal/updater"
)

// ListTags fetches tags for the given image (auth keychain, fallback anonymous).
func ListTags(ctx context.Context, imageRef *ImageRef) ([]string, error) {
	// Try with keychain, then fallback to anonymous; forward any provided crane options.
	tags, err := crane.ListTags(
		imageRef.Name,
		crane.WithAuthFromKeychain(authn.DefaultKeychain),
		crane.WithContext(ctx),
	)
	if err != nil {
		slog.Debug(
			"list tags with keychain failed, falling back to anonymous",
			"image",
			imageRef.Name,
			"err",
			err,
		)
		tags, err = crane.ListTags(
			imageRef.Name,
			crane.WithAuth(authn.Anonymous),
			crane.WithContext(ctx),
		)
		if err != nil {
			slog.Error("list tags failed", "image", imageRef.Name, "err", err)
			return nil, fmt.Errorf("list tags for %s (anonymous): %w", imageRef.Name, err)
		}
	}
	return tags, nil
}

type findLatestTagOptions struct {
	excludes      map[string]struct{}
	updateOptions []updater.Option
}

// FindLatestTagOption configures how to select the latest tag.
type FindLatestTagOption func(*findLatestTagOptions)

// WithExcludes specifies tags to exclude from consideration.
// WithExcludes specifies a set of tags to exclude from consideration.
func WithExcludes(excludes map[string]struct{}) FindLatestTagOption {
	return func(o *findLatestTagOptions) {
		o.excludes = excludes
	}
}

// WithUpdateOptions specifies options to use for version comparison.
func WithUpdateOptions(opts ...updater.Option) FindLatestTagOption {
	return func(o *findLatestTagOptions) {
		o.updateOptions = opts
	}
}

// makeFindLatestOptions creates a findLatestTagOptions struct from the provided options.
func makeFindLatestOptions(opts ...FindLatestTagOption) findLatestTagOptions {
	o := findLatestTagOptions{
		excludes: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// FindLatestTag returns the latest tag for the given image based on the provided options.
func FindLatestTag(
	ctx context.Context,
	imageRef *ImageRef,
	opts ...FindLatestTagOption,
) (string, error) {
	o := makeFindLatestOptions(opts...)
	tags, err := ListTags(ctx, imageRef)
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	bestTag := imageRef.Tag
	for _, tag := range tags {
		if _, ok := o.excludes[tag]; ok {
			slog.DebugContext(
				ctx,
				"tag excluded by exclude list",
				"tag",
				tag,
				"image",
				imageRef.String(),
			)
			continue
		}
		cmp, err := updater.Compare(bestTag, tag, o.updateOptions...)
		if err != nil {
			return "", fmt.Errorf("compare tags: %w", err)
		}
		switch cmp {
		case updater.Equal:
			slog.DebugContext(
				ctx,
				"tag is equal to baseline",
				"tag",
				tag,
				"image",
				imageRef.String(),
			)
		case updater.Greater:
			bestTag = tag
		case updater.Less:
			slog.DebugContext(
				ctx,
				"tag is less than baseline",
				"tag",
				tag,
				"image",
				imageRef.String(),
			)
		}
	}
	return bestTag, nil
}
