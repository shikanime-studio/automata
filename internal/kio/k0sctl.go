package kio

import (
	"log/slog"
	"strings"

	"github.com/shikanime-studio/automata/internal/helm"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func UpdateK0sctl(path string) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    path,
				MatchFilesGlob: []string{"cluster.yaml"},
			},
		},
		Filters: []kio.Filter{
			UpdateK0sctlCharts(),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{PackagePath: path},
		},
	}
}

func UpdateK0sctlCharts() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, node := range nodes {
			if err := updateK0sctlNode(node); err != nil {
				return nil, err
			}
		}
		return nodes, nil
	})
}

func updateK0sctlNode(node *yaml.RNode) error {
	repos := map[string]string{}
	reposNode, err := node.Pipe(yaml.Lookup("spec", "k0s", "config", "spec", "extensions", "helm", "repositories"))
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
	chartsNode, err := node.Pipe(yaml.Lookup("spec", "k0s", "config", "spec", "extensions", "helm", "charts"))
	if err != nil || chartsNode == nil {
		return nil
	}
	charts, err := chartsNode.Elements()
	if err != nil {
		return nil
	}
	for _, ch := range charts {
		if err := UpdateK0sctlChart(ch, repos); err != nil {
			slog.Warn("chart update failed", "err", err)
		}
	}
	return nil
}

func UpdateK0sctlChart(ch *yaml.RNode, repos map[string]string) error {
	repoNode, err := ch.Pipe(yaml.Get("repo"))
	if err != nil {
		slog.Warn("lookup repo failed", "err", err)
		return nil
	}
	chartNode, err := ch.Pipe(yaml.Get("chart"))
	if err != nil || yaml.GetValue(chartNode) == "" {
		chartNode, err = ch.Pipe(yaml.Get("chartname"))
		if err != nil || yaml.GetValue(chartNode) == "" {
			chartNode, err = ch.Pipe(yaml.Get("chartName"))
			if err != nil || yaml.GetValue(chartNode) == "" {
				slog.Warn("lookup chart failed", "err", err)
				return nil
			}
		}
	}
	nameNode, err := ch.Pipe(yaml.Get("name"))
	if err != nil {
		slog.Warn("lookup name failed", "err", err)
		return nil
	}
	repoVal := yaml.GetValue(repoNode)
	chartRaw := yaml.GetValue(chartNode)
	chartName := chartRaw
	if chartName == "" {
		chartName = yaml.GetValue(nameNode)
	}
	if chartName == "" && repoVal == "" {
		return nil
	}
	if chartName != "" {
		parts := strings.Split(chartName, "/")
		chartName = parts[len(parts)-1]
	}
	versionNode, err := ch.Pipe(yaml.Get("version"))
	if err != nil {
		slog.Warn("lookup version failed", "err", err)
		return nil
	}
	currVer := yaml.GetValue(versionNode)
	repoKey := repoVal
	if repoKey == "" {
		parts := strings.Split(chartRaw, "/")
		if len(parts) > 1 {
			repoKey = parts[0]
		}
	}
	resolvedRepoURL := repoKey
	if resolvedRepoURL == "" || !strings.Contains(resolvedRepoURL, "://") {
		if u, ok := repos[repoKey]; ok {
			resolvedRepoURL = u
		}
	}
	if resolvedRepoURL == "" || !strings.Contains(resolvedRepoURL, "://") {
		slog.Warn("repository URL not resolved", "chart", chartName, "repo_key", repoKey)
		return nil
	}
	ref := &helm.ChartRef{RepoURL: resolvedRepoURL, Name: chartName, Version: currVer}
	ver, err := helm.FindLatestVersion(ref)
	if err != nil || ver == "" {
		if err != nil {
			slog.Warn("failed to fetch chart version", "chart", chartName, "repo", resolvedRepoURL, "error", err)
		}
		return nil
	}
	if err := ch.PipeE(yaml.SetField("version", yaml.NewStringRNode(ver))); err != nil {
		slog.Warn("set version failed", "err", err)
		return nil
	}
	slog.Info("updated chart version", "chart", chartName, "version", ver, "repo", resolvedRepoURL)
	return nil
}
