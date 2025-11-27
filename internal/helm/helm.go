// Package helm provides helpers to query Helm repositories and resolve chart versions.
package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/shikanime-studio/automata/internal/updater"
)

// ChartRef identifies a Helm chart by repository URL, chart name, and version.
type ChartRef struct {
	RepoURL string
	Name    string
	Version string
}

// ListVersions returns all versions available for the given chart in the repo.
func ListVersions(ctx context.Context, chart *ChartRef) ([]string, error) {
	repoAdd := exec.CommandContext(
		ctx,
		"helm",
		"repo",
		"add",
		chart.Name,
		chart.RepoURL,
		"--force-update",
	)
	repoAdd.Env = os.Environ()
	if err := repoAdd.Run(); err != nil {
		return nil, fmt.Errorf("helm repo add failed: %w", err)
	}
	repoUpdate := exec.CommandContext(ctx, "helm", "repo", "update")
	repoUpdate.Env = os.Environ()
	if err := repoUpdate.Run(); err != nil {
		return nil, fmt.Errorf("helm repo update failed: %w", err)
	}
	search := exec.CommandContext(
		ctx,
		"helm",
		"search",
		"repo",
		chart.Name,
		"--output",
		"json",
		"--versions",
	)
	out, err := search.Output()
	if err != nil {
		return nil, fmt.Errorf("helm search repo failed: %w", err)
	}

	var list []map[string]any
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("helm search repo unmarshal failed: %w", err)
	}
	vers := make([]string, 0, len(list))
	for _, it := range list {
		if v, ok := it["version"].(string); ok && v != "" {
			vers = append(vers, v)
		}
	}
	return vers, nil
}

type findLatestOptions struct {
	excludes      map[string]struct{}
	updateOptions []updater.Option
}

// FindLatestOption configures the search for the latest chart version.
type FindLatestOption func(*findLatestOptions)

// WithExcludes specifies a list of versions to exclude from the search.
func WithExcludes(excludes map[string]struct{}) FindLatestOption {
	return func(o *findLatestOptions) {
		o.excludes = excludes
	}
}

// WithUpdateOptions specifies options to use for version comparison.
func WithUpdateOptions(opts ...updater.Option) FindLatestOption {
	return func(o *findLatestOptions) {
		o.updateOptions = opts
	}
}

// makeFindLatestOptions creates a findLatestOptions struct from the provided options.
func makeFindLatestOptions(opts ...FindLatestOption) findLatestOptions {
	o := findLatestOptions{
		excludes: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// FindLatestVersion chooses the best matching version according to options.
func FindLatestVersion(
	ctx context.Context,
	chart *ChartRef,
	opts ...FindLatestOption,
) (string, error) {
	o := makeFindLatestOptions(opts...)
	vers, err := ListVersions(ctx, chart)
	if err != nil {
		return "", err
	}
	bestVers := ""
	for _, v := range vers {
		if _, ok := o.excludes[v]; ok {
			slog.DebugContext(
				ctx,
				"chart version excluded by exclude list",
				"version",
				v,
				"baseline",
				chart.Version,
			)
			continue
		}
		cmp, err := updater.Compare(chart.Version, v, o.updateOptions...)
		if err != nil {
			return "", err
		}
		switch cmp {
		case updater.Equal:
			bestVers = v
		case updater.Greater:
			bestVers = v
		default:
			slog.DebugContext(
				ctx,
				"chart version excluded by strategy",
				"version",
				v,
				"baseline",
				chart.Version,
			)
		}
	}
	return bestVers, nil
}
