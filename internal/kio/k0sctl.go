package kio

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shikanime-studio/automata/internal/helm"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func UpdateK0sctlConfigs(ctx context.Context, path string) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    path,
				MatchFilesGlob: []string{"cluster.yaml"},
			},
		},
		Filters: []kio.Filter{
			UpdateK0sctlConfigsCharts(ctx),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{PackagePath: path},
		},
	}
}

func UpdateK0sctlConfigsCharts(ctx context.Context) kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := node.PipeE(UpdateK0sctlConfig(ctx)); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

func UpdateK0sctlConfig(ctx context.Context) yaml.Filter {
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
		if err != nil || chartsNode == nil {
			return nil, err
		}
		charts, err := chartsNode.Elements()
		if err != nil {
			return nil, err
		}
		for _, node := range charts {
			if err := node.PipeE(UpdateK0sctlConfigchart(ctx, repos)); err != nil {
				slog.Warn("chart update failed", "err", err)
			}
		}
		return node, nil
	})
}

func UpdateK0sctlConfigchart(ctx context.Context, repos map[string]string) yaml.Filter {
	return yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
		chartNameNode, err := node.Pipe(yaml.Lookup("chart", "name"))
		if err != nil {
			return nil, fmt.Errorf("lookup chart name failed: %w", err)
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

		ref := &helm.ChartRef{RepoURL: repoURL, Name: chartName, Version: version}
		ver, err := helm.FindLatestVersion(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch chart version: %w", err)
		}
		if ver == "" {
			return node, nil
		}
		if err := node.PipeE(yaml.SetField("version", yaml.NewStringRNode(ver))); err != nil {
			return nil, fmt.Errorf("set version failed: %w", err)
		}
		slog.Info("updated chart version", "chart", chartName, "version", ver, "repo", repoURL)
		return node, nil
	})
}
