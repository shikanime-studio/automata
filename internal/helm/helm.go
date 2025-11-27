// Package helm provides helpers to query Helm repositories and resolve chart versions.
package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/shikanime-studio/automata/internal/utils"
)

// ChartRef identifies a Helm chart by repository URL, chart name, and version.
type ChartRef struct {
	RepoURL string
	Name    string
	Version string
}

// FindLatestOption configures the behavior of FindLatestVersion.
type FindLatestOption func(*findLatestOptions)

type findLatestOptions struct {
	exclude           map[string]struct{}
	updateStrategy    utils.StrategyType
	includePreRelease bool
}

func makeFindLatestOptions(opts ...FindLatestOption) *findLatestOptions {
	o := &findLatestOptions{updateStrategy: utils.FullUpdate}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithExclude excludes exact chart versions from consideration.
func WithExclude(exclude map[string]struct{}) FindLatestOption {
	return func(o *findLatestOptions) {
		o.exclude = exclude
	}
}

// WithStrategyType sets the update strategy (full/minor/patch).
func WithStrategyType(strategy utils.StrategyType) FindLatestOption {
	return func(o *findLatestOptions) {
		o.updateStrategy = strategy
	}
}

// WithPreRelease includes prerelease versions when true.
func WithPreRelease(include bool) FindLatestOption {
	return func(o *findLatestOptions) {
		o.includePreRelease = include
	}
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

// FindLatestVersion chooses the best matching version according to options.
func FindLatestVersion(
	ctx context.Context,
	chart *ChartRef,
	opts ...FindLatestOption,
) (string, error) {
	o := makeFindLatestOptions(opts...)

	var baseline string
	switch o.updateStrategy {
	case utils.FullUpdate:
		baseline = chart.Version
	case utils.MinorUpdate:
		baseline = utils.Major(chart.Version)
	case utils.PatchUpdate:
		baseline = utils.MajorMinor(chart.Version)
	default:
		baseline = chart.Version
	}

	vers, err := ListVersions(ctx, chart)
	if err != nil {
		return "", err
	}

	bestRaw := ""
	bestSem := ""
	for _, v := range vers {
		if !o.includePreRelease && utils.PreRelease(v) != "" {
			slog.Debug("prerelease chart version ignored", "version", v)
			continue
		}
		if o.exclude != nil {
			if _, ok := o.exclude[v]; ok {
				slog.Debug("chart version excluded", "version", v)
				continue
			}
		}
		if utils.Compare(v, baseline) <= 0 {
			continue
		}
		switch o.updateStrategy {
		case utils.MinorUpdate:
			if utils.Major(v) == baseline {
				if bestSem == "" || utils.Compare(v, bestSem) > 0 {
					bestSem = v
					bestRaw = v
				}
			} else {
				slog.Debug("chart version excluded by strategy", "version", v, "baseline", baseline)
			}
		case utils.PatchUpdate:
			if utils.MajorMinor(v) == baseline {
				if bestSem == "" || utils.Compare(v, bestSem) > 0 {
					bestSem = v
					bestRaw = v
				}
			} else {
				slog.Debug("chart version excluded by strategy", "version", v, "baseline", baseline)
			}
		default:
			if bestSem == "" || utils.Compare(v, bestSem) > 0 {
				bestSem = v
				bestRaw = v
			}
		}
	}

	return bestRaw, nil
}
