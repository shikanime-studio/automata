package app

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/shikanime-studio/automata/internal/helm"
	"github.com/shikanime-studio/automata/internal/utils"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// NewUpdateK0sctlCmd updates k0sctl clusters with the latest chart versions.
func NewUpdateK0sctlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "k0sctl [DIR]",
		Short: "Update k0sctl with latest chart versions",
		RunE: func(_ *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				root = args[0]
			}
			return runUpdateK0sctl(root)
		},
	}
}

func runUpdateK0sctl(root string) error {
	slog.Debug("start k0sctl update scan", "root", root)
	g := new(errgroup.Group)
	if err := utils.WalkDirWithGitignore(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isK0sctlCluster(path) {
			slog.Debug("k0sctl cluster file detected", "file", path)
			g.Go(createUpdateK0sctlJob(path))
		} else {
			slog.Debug("skipping non-k0sctl cluster file", "file", path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("scan for cluster.yaml: %w", err)
	}
	return g.Wait()
}

func createUpdateK0sctlJob(path string) func() error {
	return func() error {
		if err := createUpdateK0sctlPipeline(path).Execute(); err != nil {
			slog.Warn("skip k0sctl update", "file", path, "err", err)
			return err
		}
		slog.Debug("pipeline execution finished", "file", path)
		return nil
	}
}

func createUpdateK0sctlPipeline(path string) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    filepath.Dir(path),
				MatchFilesGlob: []string{"cluster.yaml"},
			},
		},
		Filters: []kio.Filter{
			createUpdateK0sctlFilter(),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{PackagePath: filepath.Dir(path)},
		},
	}
}

func createUpdateK0sctlFilter() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, root := range nodes {
			repos := map[string]string{}
			reposNode, rErr := root.Pipe(
				yaml.Lookup("spec", "k0s", "config", "spec", "extensions", "helm", "repositories"),
			)
			if rErr == nil {
				rElems, rEErr := reposNode.Elements()
				if rEErr == nil {
					slog.Debug("repositories entries found", "count", len(rElems))
					for _, r := range rElems {
						rNameNode, rnErr := r.Pipe(yaml.Get("name"))
						if rnErr != nil {
							continue
						}
						rURLNode, ruErr := r.Pipe(yaml.Get("url"))
						if ruErr != nil {
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
			chartsNode, cErr := root.Pipe(
				yaml.Lookup("spec", "k0s", "config", "spec", "extensions", "helm", "charts"),
			)
			if cErr != nil {
				slog.Warn("lookup charts failed", "err", cErr)
			} else {
				slog.Debug("charts node found")
				charts, eErr := chartsNode.Elements()
				if eErr == nil {
					slog.Debug("chart entries found", "count", len(charts))
					if len(charts) == 0 {
						slog.Debug("no charts to process")
					}
					for _, ch := range charts {
						repoNode, err := ch.Pipe(yaml.Get("repo"))
						if err != nil {
							slog.Warn("lookup repo failed", "err", err)
							continue
						}
						chartNode, err := ch.Pipe(yaml.Get("chart"))
						if err != nil || yaml.GetValue(chartNode) == "" {
							// Support alternate key names used by k0sctl examples
							chartNode, err = ch.Pipe(yaml.Get("chartname"))
							if err != nil || yaml.GetValue(chartNode) == "" {
								chartNode, err = ch.Pipe(yaml.Get("chartName"))
								if err != nil || yaml.GetValue(chartNode) == "" {
									slog.Warn("lookup chart failed", "err", err)
									continue
								}
							}
						}
						nameNode, err := ch.Pipe(yaml.Get("name"))
						if err != nil {
							slog.Warn("lookup name failed", "err", err)
							continue
						}
						repoVal := yaml.GetValue(repoNode)
						chartRaw := yaml.GetValue(chartNode)
						chartName := chartRaw
						if chartName == "" {
							chartName = yaml.GetValue(nameNode)
						}
						if chartName == "" && repoVal == "" {
							slog.Debug("chart entry missing repo and name")
							continue
						}
						chartName = lastSegment(chartName)
						versionNode, err := ch.Pipe(yaml.Get("version"))
						if err != nil {
							slog.Warn("lookup version failed", "err", err)
							continue
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
							continue
						}
						slog.Debug("repository resolved", "chart", chartName, "repo_key", repoKey, "repo_url", resolvedRepoURL)
						slog.Debug("chart candidate", "chart", chartName, "repo", resolvedRepoURL, "current", currVer)
						ref := &helm.ChartRef{RepoURL: resolvedRepoURL, Name: chartName, Version: currVer}
						ver, vErr := helm.FindLatestVersion(ref)
						if vErr != nil || ver == "" {
							if vErr != nil {
								slog.Warn("failed to fetch chart version", "chart", chartName, "repo", resolvedRepoURL, "error", vErr)
							} else {
								slog.Debug("no latest version found", "chart", chartName, "repo", resolvedRepoURL, "current", currVer)
							}
							continue
						}
						if ver == currVer {
							slog.Debug("chart already up-to-date", "chart", chartName, "version", ver)
						} else {
							slog.Debug("updating chart version", "chart", chartName, "from", currVer, "to", ver, "repo", resolvedRepoURL)
						}
						if err := ch.PipeE(yaml.SetField("version", yaml.NewStringRNode(ver))); err != nil {
							slog.Warn("set version failed", "err", err)
							continue
						}
						slog.Info("updated chart version", "chart", chartName, "version", ver, "repo", resolvedRepoURL)
					}
				} else {
					slog.Warn("charts elements lookup failed", "err", eErr)
				}
			}
		}
		return nodes, nil
	})
}

func lastSegment(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}

func isK0sctlCluster(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	r, err := yaml.Parse(string(b))
	if err != nil {
		return false
	}
	kindNode, err := r.Pipe(yaml.Get("kind"))
	if err != nil {
		return false
	}
	apiNode, err := r.Pipe(yaml.Get("apiVersion"))
	if err != nil {
		return false
	}
	kind := yaml.GetValue(kindNode)
	api := yaml.GetValue(apiNode)
	if strings.ToLower(kind) != "cluster" {
		return false
	}
	if !strings.Contains(strings.ToLower(api), "k0sctl") {
		return false
	}
	return true
}
