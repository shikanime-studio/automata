package kio

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shikanime-studio/automata/internal/helm"
	"github.com/shikanime-studio/automata/internal/updater"

	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// UpdateK0sctlConfigs builds a pipeline to update helm chart versions in k0sctl configs.
func UpdateK0sctlConfigs(
	ctx context.Context,
	u updater.Updater[*helm.ChartRef],
	path string,
) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    path,
				MatchFilesGlob: []string{"cluster.yaml"},
			},
		},
		Filters: []kio.Filter{
			UpdateK0sctlConfigsCharts(ctx, u),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{PackagePath: path},
		},
	}
}

// UpdateK0sctlConfigsCharts runs chart updates across all loaded config files.
func UpdateK0sctlConfigsCharts(ctx context.Context, u updater.Updater[*helm.ChartRef]) kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		g := errgroup.Group{}
		for _, node := range nodes {
			g.Go(func() error {
				if err := node.PipeE(UpdateK0sctlConfig(ctx, u)); err != nil {
					return err
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		return nodes, nil
	})
}

// UpdateK0sctlConfig updates charts inside one k0sctl configuration.
func UpdateK0sctlConfig(ctx context.Context, u updater.Updater[*helm.ChartRef]) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		repos := map[string]string{}
		reposNode, err := node.Pipe(
			yaml.Lookup("spec", "k0s", "config", "spec", "extensions", "helm", "repositories"),
		)
		if err == nil && reposNode != nil {
			elems, rErr := reposNode.Elements()
			if rErr == nil {
				for _, r := range elems {
					rNameNode, rErr := r.Pipe(yaml.Get("name"))
					if rErr != nil {
						continue
					}
					rURLNode, rErr := r.Pipe(yaml.Get("url"))
					if rErr != nil {
						continue
					}
					rName := yaml.GetValue(rNameNode)
					rURL := yaml.GetValue(rURLNode)
					if rName != "" && rURL != "" {
						repos[rName] = rURL
					}
				}
			}
		}
		chartsNode, err := node.Pipe(
			yaml.Lookup("spec", "k0s", "config", "spec", "extensions", "helm", "charts"),
		)
		if err != nil {
			return nil, err
		}
		if chartsNode == nil {
			return node, nil
		}
		charts, err := chartsNode.Elements()
		if err != nil {
			return nil, err
		}
		g := errgroup.Group{}
		for _, node := range charts {
			g.Go(func() error {
				if err := node.PipeE(UpdateK0sctlConfigchart(ctx, u, repos)); err != nil {
					slog.WarnContext(ctx, "chart update failed", "err", err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		return node, nil
	})
}

// UpdateK0sctlConfigchart updates a single chart entry version in the config.
func UpdateK0sctlConfigchart(
	ctx context.Context,
	u updater.Updater[*helm.ChartRef],
	repos map[string]string,
) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		chartNameNode, err := node.Pipe(yaml.Get("chartname"))
		if err != nil {
			return nil, fmt.Errorf("lookup chartname failed: %w", err)
		}
		chartName := yaml.GetValue(chartNameNode)
		if chartName == "" {
			return nil, fmt.Errorf("chart name is empty")
		}
		parts := strings.SplitN(chartName, "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("chart name is invalid: %s", chartName)
		}
		chartName = parts[1]
		repoName := parts[0]
		repoURL, ok := repos[repoName]
		if !ok {
			return nil, fmt.Errorf("repository URL not found for chart %s", chartName)
		}
		if repoURL == "" || !strings.Contains(repoURL, "://") {
			return nil, fmt.Errorf("repository URL is empty or invalid for chart %s", chartName)
		}

		versionNode, err := node.Pipe(yaml.Get("version"))
		if err != nil {
			return nil, fmt.Errorf("lookup version failed: %w", err)
		}
		version := yaml.GetValue(versionNode)
		if version == "" {
			version = "latest"
		}

		ref := &helm.ChartRef{RepoURL: repoURL, Name: chartName, Version: version}
		ver, err := u.Update(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch chart version: %w", err)
		}
		if ver == "" {
			return node, nil
		}
		if err := node.PipeE(yaml.SetField("version", yaml.NewStringRNode(ver))); err != nil {
			return nil, fmt.Errorf("set version failed: %w", err)
		}
		slog.InfoContext(
			ctx,
			"updated chart version",
			"chart",
			chartName,
			"version",
			ver,
			"repo",
			repoURL,
		)
		return node, nil
	})
}
