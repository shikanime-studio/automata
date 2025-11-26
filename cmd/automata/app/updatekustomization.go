package app

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/shikanime-studio/automata/internal/container"
	"github.com/shikanime-studio/automata/internal/utils"
	"github.com/spf13/cobra"
	errgrp "golang.org/x/sync/errgroup"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// NewUpdateKustomizationCmd updates kustomize image tags across a directory tree.
// It scans for kustomization.yaml files and updates image tags based on
// the images annotation configuration and chosen registry strategy.
func NewUpdateKustomizationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kustomization [DIR...]",
		Short: "Update kustomize image tags",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var g errgrp.Group
			for _, a := range args {
				r := strings.TrimSpace(a)
				if r == "" {
					continue
				}
				rr := r
				g.Go(func() error { return runUpdateKustomization(rr) })
			}
			return g.Wait()
		},
	}
}

// runUpdateKustomization executes the kustomization update across the directory tree.
func runUpdateKustomization(root string) error {
	var g errgrp.Group
	if err := utils.WalkDirWithGitignore(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !isKustomizationFile(path) {
			return nil
		}
		g.Go(createUpdateKustomizationJob(filepath.Dir(path)))
		return nil
	}); err != nil {
		return err
	}
	return g.Wait()
}

// createUpdateKustomizationJob returns a task function that updates image tags
// for a specific kustomization directory, suitable for use with errgroup.
func createUpdateKustomizationJob(path string) func() error {
	return func() error {
		if err := createUpdateKustomizationPipeline(path).Execute(); err != nil {
			slog.Warn("skip kustomization update", "dir", path, "err", err)
			return err
		}
		return nil
	}
}

// createUpdateKustomizationPipeline creates a kustomize pipeline to update image tags
// and recommended labels for images defined in the kustomization.yaml at the given directory.
func createUpdateKustomizationPipeline(path string) kio.Pipeline {
	return kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath:    path,
				MatchFilesGlob: []string{"kustomization.yaml"},
			},
		},
		Filters: []kio.Filter{
			createUpdateImagesFilter(),
			createUpdateLabelsFilter(),
		},
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{
				PackagePath: path,
			},
		},
	}
}

// createUpdateImagesFilter creates a kustomize filter to update image tags
// for images defined in the kustomization.yaml.
func createUpdateImagesFilter() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, root := range nodes {
			imageAnnotationNode, err := root.Pipe(utils.GetImagesAnnotation())
			if err != nil {
				return nil, fmt.Errorf("get images annotation: %w", err)
			}
			imageConfigs, err := utils.GetImagesConfig(imageAnnotationNode)
			if err != nil {
				return nil, fmt.Errorf("get image config: %w", err)
			}
			imageConfigsByName := utils.CreateImageConfigsByName(imageConfigs)

			imagesNode, err := root.Pipe(yaml.Lookup("images"))
			if err != nil {
				return nil, fmt.Errorf("lookup images: %w", err)
			}
			imageNodes, err := imagesNode.Elements()
			if err != nil {
				return nil, fmt.Errorf("get images elements: %w", err)
			}

			// Update image tags based on annotation configs
			for _, img := range imageNodes {
				nameNode, err := img.Pipe(yaml.Get("name"))
				if err != nil {
					slog.Warn("missing name in images entry", "err", err)
					continue
				}
				name := yaml.GetValue(nameNode)
				cfg, ok := imageConfigsByName[name]
				if !ok {
					continue
				}
				newNameNode, err := img.Pipe(yaml.Get("newName"))
				if err != nil {
					return nil, fmt.Errorf("get newName for %s: %w", name, err)
				}

				options := []container.FindLatestOption{}

				if cfg.StrategyType != utils.FullUpdate {
					options = append(options, container.WithStrategyType(cfg.StrategyType))
				}

				if len(cfg.ExcludeTags) > 0 {
					options = append(options, container.WithExclude(cfg.ExcludeTags))
				}

				if cfg.TagRegex != nil {
					options = append(options, container.WithTransform(cfg.TagRegex))
				}

				imageRef := container.ImageRef{
					Name: yaml.GetValue(newNameNode),
				}

				currentTagNode, err := img.Pipe(yaml.Get("newTag"))
				if err != nil {
					return nil, fmt.Errorf("get current newTag for %s: %w", name, err)
				}
				currentTag := yaml.GetValue(currentTagNode)
				if currentTag != "" {
					var version string
					version, err = utils.ParseSemverWithRegex(cfg.TagRegex, currentTag)
					if err != nil {
						return nil, fmt.Errorf("parse semver for %s: %w", currentTag, err)
					}
					imageRef.Tag = version
				}

				latest, err := container.FindLatestTag(&imageRef, options...)
				if err != nil {
					latest = currentTag
				}
				if latest == "" {
					slog.Info("no matching tag found", "image", name)
					continue
				}
				if err = img.PipeE(yaml.SetField("newTag", yaml.NewStringRNode(latest))); err != nil {
					return nil, fmt.Errorf("set newTag for %s: %w", name, err)
				}
				slog.Info(
					"updated image tag",
					"name",
					name,
					"image",
					imageRef.String(),
					"tag",
					latest,
				)
			}
		}
		return nodes, nil
	})
}

// createUpdateLabelsFilter creates a kustomize filter to update recommended labels
// for images defined in the kustomization.yaml.
func createUpdateLabelsFilter() kio.Filter {
	return kio.FilterFunc(func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, root := range nodes {
			imageAnnotationNode, err := root.Pipe(utils.GetImagesAnnotation())
			if err != nil {
				return nil, fmt.Errorf("get images annotation: %w", err)
			}
			imageConfigs, err := utils.GetImagesConfig(imageAnnotationNode)
			if err != nil {
				return nil, fmt.Errorf("get image config: %w", err)
			}
			imageConfigsByName := utils.CreateImageConfigsByName(imageConfigs)

			labelsNode, err := root.Pipe(yaml.Lookup("labels"))
			if err != nil {
				return nil, fmt.Errorf("lookup labels: %w", err)
			}
			labelNodes, err := labelsNode.Elements()
			if err != nil {
				return nil, fmt.Errorf("get labels pairs: %w", err)
			}
			var recoLabelName string
			for _, labelNode := range labelNodes {
				var valNode *yaml.RNode
				valNode, err = labelNode.Pipe(
					yaml.Lookup("pairs"),
					yaml.Get(utils.KubernetesNameLabel),
				)
				if err != nil {
					return nil, fmt.Errorf("get %s: %w", utils.KubernetesNameLabel, err)
				}
				recoLabelName = yaml.GetValue(valNode)
			}

			imagesNode, err := root.Pipe(yaml.Lookup("images"))
			if err != nil {
				return nil, fmt.Errorf("lookup images: %w", err)
			}
			imageNodes, err := imagesNode.Elements()
			if err != nil {
				return nil, fmt.Errorf("get images elements: %w", err)
			}

			// Update recommended labels based on updated image tags
			for _, img := range imageNodes {
				nameNode, err := img.Pipe(yaml.Get("name"))
				if err != nil {
					slog.Warn("missing name in images entry", "err", err)
					continue
				}
				name := yaml.GetValue(nameNode)
				cfg, ok := imageConfigsByName[name]
				if !ok {
					continue
				}

				if recoLabelName == name {
					// Get the current newTag (which should have been updated by the images filter)
					currentTagNode, err := img.Pipe(yaml.Get("newTag"))
					if err != nil {
						return nil, fmt.Errorf("get current newTag for %s: %w", name, err)
					}
					latest := yaml.GetValue(currentTagNode)
					if latest == "" {
						continue
					}

					var vers string
					if cfg.TagRegex != nil {
						vers, err = utils.ParseSemverWithRegex(cfg.TagRegex, latest)
						if err != nil {
							return nil, fmt.Errorf("parse semver for %s: %w", latest, err)
						}
					}
					if err = root.PipeE(utils.SetRecommandedLabels(name, utils.Canonical(vers))); err != nil {
						return nil, fmt.Errorf("set %s: %w", utils.KubernetesVersionLabel, err)
					}
					slog.Info(
						"updated recommended labels",
						"name",
						name,
						"image",
						name,
						"tag",
						latest,
					)
				}
			}
		}
		return nodes, nil
	})
}

// isKustomizationFile reports whether the path is a kustomization.yaml.
func isKustomizationFile(path string) bool {
	return filepath.Base(path) == "kustomization.yaml"
}
